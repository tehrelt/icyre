// Package redis opens Redis clients and provides key naming and a typed TTL
// cache. Redis is never a source of truth (specs/data/redis.md): everything
// stored through this package must be rebuildable or safe to lose.
package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config configures a client.
type Config struct {
	Addr     string
	Username string
	Password string
	DB       int
	// PoolSize is the maximum number of connections (default 10 per CPU).
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// PoolTimeout bounds waiting for a free connection.
	PoolTimeout time.Duration
}

// Client is the go-redis client used across services.
type Client = goredis.Client

// Nil is returned when a key does not exist.
var Nil = goredis.Nil

// Open connects and pings Redis. Timeouts are short by default: Redis is a
// cache, callers must degrade rather than wait.
func Open(ctx context.Context, cfg Config) (*Client, error) {
	withDefault := func(d, def time.Duration) time.Duration {
		if d <= 0 {
			return def
		}
		return d
	}
	cl := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  withDefault(cfg.DialTimeout, 2*time.Second),
		ReadTimeout:  withDefault(cfg.ReadTimeout, 500*time.Millisecond),
		WriteTimeout: withDefault(cfg.WriteTimeout, 500*time.Millisecond),
		PoolTimeout:  withDefault(cfg.PoolTimeout, time.Second),
	})
	if err := cl.Ping(ctx).Err(); err != nil {
		_ = cl.Close()
		return nil, fmt.Errorf("ping redis %s: %w", cfg.Addr, err)
	}
	return cl, nil
}

// Check returns a readiness check.
func Check(cl *Client) func(context.Context) error {
	return func(ctx context.Context) error { return cl.Ping(ctx).Err() }
}

// Key joins parts with ":" following the convention
//
//	<namespace>:<entity>[:<qualifier>]:<id>
//
// e.g. Key("cache", "track", id), Key("session", id), Key("ratelimit", "login", subject, window).
// Parts must not contain ":" — IDs are UUIDs or slugs.
func Key(parts ...string) string {
	return strings.Join(parts, ":")
}
