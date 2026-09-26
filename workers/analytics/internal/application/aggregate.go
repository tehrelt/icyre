package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// AggregateStore recomputes the daily aggregates of one UTC day from the raw
// events. Recomputing is idempotent: the newest computation wins.
type AggregateStore interface {
	RecomputeDay(ctx context.Context, day time.Time) error
}

// Aggregator runs the aggregation jobs.
type Aggregator struct {
	store    AggregateStore
	lookback int
	now      func() time.Time
	log      *slog.Logger
	// observe, when set, receives each run's outcome (for metrics).
	observe func(err error, took time.Duration)
}

// NewAggregator recomputes today and lookback days before it on every run.
func NewAggregator(s AggregateStore, lookback int, log *slog.Logger, observe func(error, time.Duration)) *Aggregator {
	return &Aggregator{store: s, lookback: max(lookback, 0), now: time.Now, log: log, observe: observe}
}

// Recompute rebuilds the aggregates of every UTC day in [from, to].
func (a *Aggregator) Recompute(ctx context.Context, from, to time.Time) error {
	from, to = Day(from), Day(to)
	if to.Before(from) {
		return fmt.Errorf("aggregate: %s is after %s", from.Format(time.DateOnly), to.Format(time.DateOnly))
	}
	var errs []error
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if err := a.store.RecomputeDay(ctx, d); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			errs = append(errs, fmt.Errorf("%s: %w", d.Format(time.DateOnly), err))
		}
	}
	return errors.Join(errs...)
}

// RecomputeRecent rebuilds today and the lookback days: late and redelivered
// events of those days are picked up.
func (a *Aggregator) RecomputeRecent(ctx context.Context) error {
	today := Day(a.now())
	return a.Recompute(ctx, today.AddDate(0, 0, -a.lookback), today)
}

// Run recomputes the recent days every period until ctx is cancelled. A
// failed run is logged and retried on the next tick.
func (a *Aggregator) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		start := time.Now()
		err := a.RecomputeRecent(ctx)
		if ctx.Err() != nil {
			return
		}
		if a.observe != nil {
			a.observe(err, time.Since(start))
		}
		if err != nil {
			a.log.ErrorContext(ctx, "aggregation failed", "error", err)
		} else {
			a.log.DebugContext(ctx, "aggregates recomputed", "took", time.Since(start).String())
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Day is the UTC day containing t.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
