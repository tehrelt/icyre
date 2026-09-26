package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/platform/kafka"
)

// Producer publishes messages and waits for the broker's acknowledgement.
type Producer interface {
	Publish(ctx context.Context, msgs ...kafka.Message) error
}

// RelayConfig tunes a Relay.
type RelayConfig struct {
	// Table is <schema>.outbox.
	Table string
	// Batch bounds the rows published per transaction (default 100).
	Batch int
	// Interval is the idle poll period (default 200ms); a full batch is
	// followed by the next one right away.
	Interval time.Duration
	// RetryBackoff is the pause after a failed batch (default 1s).
	RetryBackoff time.Duration
}

func (c RelayConfig) withDefaults() RelayConfig {
	if c.Batch <= 0 {
		c.Batch = 100
	}
	if c.Interval <= 0 {
		c.Interval = 200 * time.Millisecond
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = time.Second
	}
	return c
}

// Relay moves outbox rows to Kafka.
//
// Only one relay per table publishes at a time (a transaction-scoped
// advisory lock), so rows go out in insertion order and per-key order holds
// across service replicas; the others stay idle until it is released.
type Relay struct {
	pool     *pgxpool.Pool
	producer Producer
	cfg      RelayConfig
	log      *slog.Logger
	metrics  *relayMetrics
}

// NewRelay returns a Relay. reg may be nil.
func NewRelay(pool *pgxpool.Pool, p Producer, cfg RelayConfig, log *slog.Logger, reg prometheus.Registerer) (*Relay, error) {
	if err := checkTable(cfg.Table); err != nil {
		return nil, err
	}
	return &Relay{pool: pool, producer: p, cfg: cfg.withDefaults(), log: log, metrics: newRelayMetrics(reg, cfg.Table)}, nil
}

// Run relays until ctx is done.
func (r *Relay) Run(ctx context.Context) error {
	for {
		n, err := r.RelayOnce(ctx)
		wait := r.cfg.Interval
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			r.log.ErrorContext(ctx, "outbox relay failed", "table", r.cfg.Table, "error", err)
			wait = r.cfg.RetryBackoff
		case n == r.cfg.Batch:
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

type row struct {
	id      int64
	msg     kafka.Message
	created time.Time
}

// RelayOnce publishes up to one batch and returns how many rows it sent.
// Another relay holding the table lock makes it a no-op.
func (r *Relay) RelayOnce(ctx context.Context) (int, error) {
	sent := 0
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var leader bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtext($1))`, r.cfg.Table).Scan(&leader); err != nil {
			return fmt.Errorf("outbox lock: %w", err)
		}
		if !leader {
			return nil
		}
		rows, err := r.pending(ctx, tx)
		if err != nil || len(rows) == 0 {
			return err
		}
		msgs := make([]kafka.Message, len(rows))
		ids := make([]int64, len(rows))
		for i, rw := range rows {
			msgs[i], ids[i] = rw.msg, rw.id
		}
		if err := r.producer.Publish(ctx, msgs...); err != nil {
			r.metrics.failed()
			return fmt.Errorf("outbox publish: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM `+r.cfg.Table+` WHERE id = ANY($1)`, ids); err != nil {
			return fmt.Errorf("outbox delete: %w", err)
		}
		sent = len(rows)
		r.metrics.published(sent, time.Since(rows[0].created))
		return nil
	})
	if err != nil {
		return 0, err
	}
	return sent, nil
}

func (r *Relay) pending(ctx context.Context, tx pgx.Tx) ([]row, error) {
	rs, err := tx.Query(ctx, `SELECT id, topic, key, value, headers, created_at FROM `+r.cfg.Table+` ORDER BY id LIMIT $1`, r.cfg.Batch)
	if err != nil {
		return nil, fmt.Errorf("outbox select: %w", err)
	}
	return pgx.CollectRows(rs, func(cr pgx.CollectableRow) (row, error) {
		var (
			rw  row
			raw []byte
		)
		if err := cr.Scan(&rw.id, &rw.msg.Topic, &rw.msg.Key, &rw.msg.Value, &raw, &rw.created); err != nil {
			return rw, err
		}
		if err := json.Unmarshal(raw, &rw.msg.Headers); err != nil {
			return rw, errors.Join(fmt.Errorf("outbox row %d headers", rw.id), err)
		}
		return rw, nil
	})
}

type relayMetrics struct {
	sent   prometheus.Counter
	errors prometheus.Counter
	lag    prometheus.Gauge
}

func newRelayMetrics(reg prometheus.Registerer, table string) *relayMetrics {
	if reg == nil {
		return nil
	}
	labels := prometheus.Labels{"table": table}
	m := &relayMetrics{
		sent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "outbox_published_total", Help: "Outbox messages published to Kafka.", ConstLabels: labels,
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "outbox_publish_errors_total", Help: "Failed outbox publish batches.", ConstLabels: labels,
		}),
		lag: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "outbox_lag_seconds", Help: "Age of the oldest message in the last published batch.", ConstLabels: labels,
		}),
	}
	reg.MustRegister(m.sent, m.errors, m.lag)
	return m
}

func (m *relayMetrics) published(n int, lag time.Duration) {
	if m == nil {
		return
	}
	m.sent.Add(float64(n))
	m.lag.Set(lag.Seconds())
}

func (m *relayMetrics) failed() {
	if m != nil {
		m.errors.Inc()
	}
}
