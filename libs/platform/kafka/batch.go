package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// BatchConfig bounds one batch of a BatchConsumer.
type BatchConfig struct {
	// MaxRecords flushes once this many records are collected (default 1000).
	MaxRecords int
	// Linger flushes this long after the first record of a batch at the
	// latest (default 1s).
	Linger time.Duration
}

// Decoder turns one record into a batch item. It must not depend on other
// systems: an error is treated as permanent — the record goes to the DLQ,
// or, without one, consumption stops. ErrSkip drops the record (e.g. an
// event type the consumer does not care about).
type Decoder[T any] func(ctx context.Context, rec Record) (T, error)

// ErrSkip, returned by a Decoder, commits the record without flushing it.
var ErrSkip = errors.New("skip record")

// Flusher writes one batch. It must be idempotent: after a crash the batch
// is redelivered, possibly split differently. Errors are retried with
// backoff up to MaxRetries; then consumption stops without committing, and
// the batch is redelivered after a restart.
type Flusher[T any] func(ctx context.Context, items []T) error

// BatchConsumer collects records into batches for sinks that want few large
// writes (e.g. ClickHouse) and commits offsets only after a batch is flushed.
type BatchConsumer[T any] struct {
	c      *Consumer
	bc     BatchConfig
	decode Decoder[T]
	flush  Flusher[T]
}

// NewBatchConsumer creates a consumer group member that flushes batches.
// metrics may be nil.
func NewBatchConsumer[T any](cfg ConsumerConfig, bc BatchConfig, decode Decoder[T], flush Flusher[T], log *slog.Logger, metrics *ConsumerMetrics) (*BatchConsumer[T], error) {
	if bc.MaxRecords <= 0 {
		bc.MaxRecords = 1000
	}
	if bc.Linger <= 0 {
		bc.Linger = time.Second
	}
	c, err := NewConsumer(cfg, nil, log, metrics)
	if err != nil {
		return nil, err
	}
	return &BatchConsumer[T]{c: c, bc: bc, decode: decode, flush: flush}, nil
}

// Run consumes until ctx is cancelled. A batch still being collected on
// shutdown is dropped uncommitted and redelivered later.
func (b *BatchConsumer[T]) Run(ctx context.Context) error {
	defer b.c.cl.Close()
	for {
		recs := b.collect(ctx)
		if ctx.Err() != nil {
			break
		}
		if len(recs) == 0 {
			continue
		}
		if err := b.handle(ctx, recs); err != nil {
			return err
		}
		cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := b.c.cl.CommitRecords(cctx, recs...); err != nil {
			b.c.log.ErrorContext(ctx, "kafka commit failed", "error", err)
		}
		cancel()
	}
	b.c.log.Info("kafka batch consumer stopped")
	return nil
}

// collect blocks for the first records, then keeps polling until the batch
// is full or Linger has passed.
func (b *BatchConsumer[T]) collect(ctx context.Context) []*kgo.Record {
	recs := b.poll(ctx, b.bc.MaxRecords)
	if len(recs) == 0 {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, b.bc.Linger)
	defer cancel()
	for len(recs) < b.bc.MaxRecords && lctx.Err() == nil {
		recs = append(recs, b.poll(lctx, b.bc.MaxRecords-len(recs))...)
	}
	return recs
}

func (b *BatchConsumer[T]) poll(ctx context.Context, max int) []*kgo.Record {
	fetches := b.c.cl.PollRecords(ctx, max)
	fetches.EachError(func(topic string, p int32, err error) {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return
		}
		b.c.log.ErrorContext(ctx, "kafka fetch error", "topic", topic, "partition", p, "error", err)
	})
	return fetches.Records()
}

// handle decodes every record (dead-lettering the undecodable ones) and
// flushes the rest with retries.
func (b *BatchConsumer[T]) handle(ctx context.Context, recs []*kgo.Record) error {
	items := make([]T, 0, len(recs))
	var flushed []*kgo.Record
	for _, r := range recs {
		rec := toRecord(r)
		item, err := b.decode(ctx, rec)
		if err == nil {
			items = append(items, item)
			flushed = append(flushed, r)
			continue
		}
		if errors.Is(err, ErrSkip) {
			b.c.count(r.Topic, "skip")
			continue
		}
		b.c.log.ErrorContext(ctx, "kafka message undecodable", "topic", r.Topic, "partition", r.Partition, "offset", r.Offset, "error", err)
		if !b.c.cfg.DLQ {
			b.c.count(r.Topic, "error")
			return fmt.Errorf("decode %s/%d@%d: %w", r.Topic, r.Partition, r.Offset, err)
		}
		if err := b.c.toDLQ(ctx, rec, err); err != nil {
			return err
		}
	}
	if len(items) == 0 {
		return nil
	}

	var err error
	for attempt := 0; attempt <= b.c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			for _, r := range flushed {
				b.c.count(r.Topic, "retry")
			}
			select {
			case <-time.After(b.c.cfg.RetryBackoff * time.Duration(attempt)):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		start := time.Now()
		if err = b.flush(ctx, items); err == nil {
			for _, r := range flushed {
				b.c.count(r.Topic, "ok")
				if b.c.metrics != nil {
					// Per-record share of the batch, comparable with Consumer.
					b.c.metrics.duration.WithLabelValues(r.Topic, b.c.cfg.Group).Observe(time.Since(start).Seconds() / float64(len(flushed)))
				}
			}
			return nil
		}
		b.c.log.WarnContext(ctx, "kafka batch flush failed", "records", len(items), "attempt", attempt+1, "error", err)
	}
	for _, r := range flushed {
		b.c.count(r.Topic, "error")
	}
	return fmt.Errorf("flush batch of %d: %w", len(items), err)
}

func toRecord(r *kgo.Record) Record {
	return Record{
		Topic: r.Topic, Partition: r.Partition, Offset: r.Offset,
		Key: r.Key, Value: r.Value, Headers: headersToMap(r.Headers), Timestamp: r.Timestamp,
	}
}
