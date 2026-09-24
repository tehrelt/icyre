// Package postgres opens instrumented pgx connection pools and runs
// per-schema migrations.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// Config configures a connection pool.
type Config struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
	// StatementTimeout is applied server-side to every session (0 = none).
	StatementTimeout time.Duration
	// ApplicationName shows up in pg_stat_activity.
	ApplicationName string
}

// Open creates a pool, verifies connectivity and optionally registers
// query and pool metrics in reg (nil disables metrics).
func Open(ctx context.Context, cfg Config, reg prometheus.Registerer) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	if cfg.MaxConns > 0 {
		pc.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pc.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pc.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		pc.HealthCheckPeriod = cfg.HealthCheckPeriod
	}
	if cfg.ConnectTimeout > 0 {
		pc.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}
	if cfg.ApplicationName != "" {
		pc.ConnConfig.RuntimeParams["application_name"] = cfg.ApplicationName
	}
	if cfg.StatementTimeout > 0 {
		pc.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	}

	var qm *queryMetrics
	if reg != nil {
		qm = newQueryMetrics(reg)
	}
	pc.ConnConfig.Tracer = newTracer(qm)

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx := ctx
	if cfg.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
		defer cancel()
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if reg != nil {
		if err := reg.Register(newPoolCollector(pool)); err != nil {
			pool.Close()
			return nil, fmt.Errorf("register pool metrics: %w", err)
		}
	}
	return pool, nil
}

// Check returns a readiness check that pings the pool.
func Check(pool *pgxpool.Pool) func(context.Context) error {
	return func(ctx context.Context) error { return pool.Ping(ctx) }
}
