package kafka

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ProducerConfig configures a Producer.
type ProducerConfig struct {
	Brokers  []string
	ClientID string
}

// Producer publishes records synchronously with acks=all and idempotence.
type Producer struct {
	cl       *kgo.Client
	produced *prometheus.CounterVec
}

// NewProducer connects a producer. reg may be nil to disable metrics.
func NewProducer(cfg ProducerConfig, reg prometheus.Registerer) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.SnappyCompression(), kgo.NoCompression()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	p := &Producer{cl: cl}
	if reg != nil {
		p.produced = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_producer_messages_total",
			Help: "Messages produced by topic and result.",
		}, []string{"topic", "result"})
		if err := reg.Register(p.produced); err != nil {
			cl.Close()
			return nil, fmt.Errorf("register producer metrics: %w", err)
		}
	}
	return p, nil
}

// Publish sends messages and waits for broker acknowledgement. The trace
// context of ctx travels in the record headers.
func (p *Producer) Publish(ctx context.Context, msgs ...Message) error {
	recs := make([]*kgo.Record, 0, len(msgs))
	for _, m := range msgs {
		headers := make(map[string]string, len(m.Headers)+2)
		for k, v := range m.Headers {
			headers[k] = v
		}
		injectTrace(ctx, headers)
		recs = append(recs, &kgo.Record{Topic: m.Topic, Key: m.Key, Value: m.Value, Headers: mapToHeaders(headers)})
	}

	results := p.cl.ProduceSync(ctx, recs...)
	for _, r := range results {
		result := "ok"
		if r.Err != nil {
			result = "error"
		}
		if p.produced != nil {
			p.produced.WithLabelValues(r.Record.Topic, result).Inc()
		}
	}
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("kafka produce: %w", err)
	}
	return nil
}

// Ping checks broker connectivity (readiness).
func (p *Producer) Ping(ctx context.Context) error { return p.cl.Ping(ctx) }

// Close flushes buffered records and closes the client.
func (p *Producer) Close(ctx context.Context) error {
	err := p.cl.Flush(ctx)
	p.cl.Close()
	return err
}
