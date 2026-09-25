package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// CacheMetrics counts cache results per cache name (low cardinality: the
// name is a code constant like "track", never a key).
type CacheMetrics struct {
	requests *prometheus.CounterVec
}

// NewCacheMetrics registers redis_cache_requests_total{cache,result}.
func NewCacheMetrics(reg prometheus.Registerer) *CacheMetrics {
	m := &CacheMetrics{requests: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "redis_cache_requests_total",
		Help: "Cache lookups by cache name and result (hit, miss, error).",
	}, []string{"cache", "result"})}
	reg.MustRegister(m.requests)
	return m
}

// Cache is a JSON TTL cache over one key namespace. It holds no business
// types: callers choose T.
type Cache[T any] struct {
	cl      *Client
	name    string
	ttl     time.Duration
	log     *slog.Logger
	metrics *CacheMetrics
}

// NewCache returns a cache whose keys are "cache:<name>:<id>". ttl must be
// positive: a cache entry always expires (specs/data/redis.md).
func NewCache[T any](cl *Client, name string, ttl time.Duration, log *slog.Logger, m *CacheMetrics) *Cache[T] {
	if ttl <= 0 {
		panic("redis cache ttl must be positive")
	}
	return &Cache[T]{cl: cl, name: name, ttl: ttl, log: log, metrics: m}
}

func (c *Cache[T]) key(id string) string { return Key("cache", c.name, id) }

func (c *Cache[T]) count(result string) {
	if c.metrics != nil {
		c.metrics.requests.WithLabelValues(c.name, result).Inc()
	}
}

// Get returns the cached value and whether it was found. Redis errors are
// reported as misses plus the error, so callers can fall back to the source.
func (c *Cache[T]) Get(ctx context.Context, id string) (T, bool, error) {
	var zero T
	raw, err := c.cl.Get(ctx, c.key(id)).Bytes()
	switch {
	case errors.Is(err, Nil):
		c.count("miss")
		return zero, false, nil
	case err != nil:
		c.count("error")
		return zero, false, fmt.Errorf("cache %s get: %w", c.name, err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		c.count("error")
		return zero, false, fmt.Errorf("cache %s decode: %w", c.name, err)
	}
	c.count("hit")
	return v, true, nil
}

// Set stores v with the cache TTL.
func (c *Cache[T]) Set(ctx context.Context, id string, v T) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache %s encode: %w", c.name, err)
	}
	if err := c.cl.Set(ctx, c.key(id), raw, c.ttl).Err(); err != nil {
		return fmt.Errorf("cache %s set: %w", c.name, err)
	}
	return nil
}

// Delete invalidates an entry.
func (c *Cache[T]) Delete(ctx context.Context, id string) error {
	return c.cl.Del(ctx, c.key(id)).Err()
}

// GetOrLoad returns the cached value or loads, stores and returns it.
// A Redis failure never fails the call: the loader is the source of truth.
func (c *Cache[T]) GetOrLoad(ctx context.Context, id string, load func(context.Context) (T, error)) (T, error) {
	if v, ok, err := c.Get(ctx, id); err == nil && ok {
		return v, nil
	} else if err != nil && c.log != nil {
		c.log.WarnContext(ctx, "cache read failed, using source", "cache", c.name, "error", err)
	}
	v, err := load(ctx)
	if err != nil {
		return v, err
	}
	if err := c.Set(ctx, id, v); err != nil && c.log != nil {
		c.log.WarnContext(ctx, "cache write failed", "cache", c.name, "error", err)
	}
	return v, nil
}
