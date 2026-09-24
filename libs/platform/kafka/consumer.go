package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Handler processes one record. It must be idempotent: the same record can
// be delivered more than once. Returning an error triggers a retry; after
// MaxRetries the record goes to the DLQ (or consumption stops without one).
type Handler func(ctx context.Context, rec Record) error

// ErrPermanent marks an error that must not be retried (e.g. undecodable
// payload). Wrap it: fmt.Errorf("decode: %w", kafka.ErrPermanent).
var ErrPermanent = errors.New("permanent failure")

// ConsumerConfig configures a Consumer.
type ConsumerConfig struct {
	Brokers  []string
	ClientID string
	// Group is the consumer group; one per logical consumer.
	Group        string
	Topics       []string
	MaxRetries   int
	RetryBackoff time.Duration
	// DLQ sends records that exhausted retries to <topic>.dlq.
	DLQ bool
}

// ConsumerMetrics are shared by all consumers of a service.
type ConsumerMetrics struct {
	messages *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewConsumerMetrics registers kafka_consumer_* metrics.
func NewConsumerMetrics(reg prometheus.Registerer) *ConsumerMetrics {
	m := &ConsumerMetrics{
		messages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_consumer_messages_total",
			Help: "Consumed messages by topic, group and result (ok, retry, dlq, error).",
		}, []string{"topic", "group", "result"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kafka_consumer_handle_duration_seconds",
			Help:    "Time spent handling one message.",
			Buckets: prometheus.DefBuckets,
		}, []string{"topic", "group"}),
	}
	reg.MustRegister(m.messages, m.duration)
	return m
}

// Consumer runs a Handler for every record of its topics.
type Consumer struct {
	cfg     ConsumerConfig
	cl      *kgo.Client
	handler Handler
	log     *slog.Logger
	metrics *ConsumerMetrics
}

// NewConsumer creates a consumer group member. metrics may be nil.
func NewConsumer(cfg ConsumerConfig, h Handler, log *slog.Logger, metrics *ConsumerMetrics) (*Consumer, error) {
	if cfg.Group == "" || len(cfg.Topics) == 0 {
		return nil, errors.New("kafka consumer: group and topics are required")
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 500 * time.Millisecond
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.ConsumerGroup(cfg.Group),
		kgo.ConsumeTopics(cfg.Topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return &Consumer{cfg: cfg, cl: cl, handler: h, log: log.With("group", cfg.Group), metrics: metrics}, nil
}

// Run consumes until ctx is cancelled. The record in flight is finished and
// committed; then the member leaves the group and the client closes.
func (c *Consumer) Run(ctx context.Context) error {
	defer c.cl.Close()
	for {
		fetches := c.cl.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			break
		}
		fetches.EachError(func(topic string, p int32, err error) {
			c.log.ErrorContext(ctx, "kafka fetch error", "topic", topic, "partition", p, "error", err)
		})

		var processed []*kgo.Record
		var stopErr error
		fetches.EachRecord(func(r *kgo.Record) {
			if stopErr != nil {
				return
			}
			if err := c.process(ctx, r); err != nil {
				stopErr = err
				return
			}
			processed = append(processed, r)
		})

		// Commit what was handled even when shutting down; use a fresh
		// context so cancellation does not drop the commit.
		if len(processed) > 0 {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := c.cl.CommitRecords(cctx, processed...); err != nil {
				c.log.ErrorContext(ctx, "kafka commit failed", "error", err)
			}
			cancel()
		}
		if stopErr != nil {
			return stopErr
		}
	}
	c.log.Info("kafka consumer stopped")
	return nil
}

func (c *Consumer) process(ctx context.Context, r *kgo.Record) error {
	rec := Record{
		Topic: r.Topic, Partition: r.Partition, Offset: r.Offset,
		Key: r.Key, Value: r.Value, Headers: headersToMap(r.Headers), Timestamp: r.Timestamp,
	}
	hctx := extractTrace(ctx, rec.Headers)

	var err error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			c.count(r.Topic, "retry")
			select {
			case <-time.After(c.cfg.RetryBackoff * time.Duration(attempt)):
			case <-ctx.Done():
				// Not committed: it will be redelivered after restart.
				return ctx.Err()
			}
		}
		start := time.Now()
		err = c.handler(hctx, rec)
		if c.metrics != nil {
			c.metrics.duration.WithLabelValues(r.Topic, c.cfg.Group).Observe(time.Since(start).Seconds())
		}
		if err == nil {
			c.count(r.Topic, "ok")
			return nil
		}
		if errors.Is(err, ErrPermanent) {
			break
		}
	}

	c.log.ErrorContext(hctx, "kafka message failed", "topic", r.Topic, "partition", r.Partition, "offset", r.Offset, "error", err)
	if !c.cfg.DLQ {
		c.count(r.Topic, "error")
		return fmt.Errorf("handle %s/%d@%d: %w", r.Topic, r.Partition, r.Offset, err)
	}
	return c.toDLQ(hctx, rec, err)
}

func (c *Consumer) toDLQ(ctx context.Context, rec Record, cause error) error {
	headers := make(map[string]string, len(rec.Headers)+4)
	for k, v := range rec.Headers {
		headers[k] = v
	}
	headers["dlq.error"] = cause.Error()
	headers["dlq.source.topic"] = rec.Topic
	headers["dlq.source.partition"] = strconv.Itoa(int(rec.Partition))
	headers["dlq.source.offset"] = strconv.FormatInt(rec.Offset, 10)

	res := c.cl.ProduceSync(ctx, &kgo.Record{
		Topic: DLQTopic(rec.Topic), Key: rec.Key, Value: rec.Value, Headers: mapToHeaders(headers),
	})
	if err := res.FirstErr(); err != nil {
		c.count(rec.Topic, "error")
		return fmt.Errorf("publish to dlq: %w", err)
	}
	c.count(rec.Topic, "dlq")
	return nil
}

func (c *Consumer) count(topic, result string) {
	if c.metrics != nil {
		c.metrics.messages.WithLabelValues(topic, c.cfg.Group, result).Inc()
	}
}
