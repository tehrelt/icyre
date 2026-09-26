// Package config loads Analytics Worker settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the worker.
const ServiceName = "analytics"

// ConsumerGroup consumes playback.events.
const ConsumerGroup = "analytics"

// Config is the complete worker configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config // health and metrics only
	ShutdownTimeout time.Duration
	ClickHouse      clickhouse.Config
	KafkaBrokers    []string
	MaxRetries      int
	RetryBackoff    time.Duration
	// BatchSize and BatchLinger bound one ClickHouse insert.
	BatchSize   int
	BatchLinger time.Duration
	CatalogURL  string
	// CatalogTimeout bounds one Catalog request; TrackCacheTTL is how long a
	// track's album and artists are reused.
	CatalogTimeout time.Duration
	TrackCacheTTL  time.Duration
	// AggregateEvery is the period of the aggregation job; it recomputes
	// today and AggregateLookbackDays before it (late events).
	AggregateEvery        time.Duration
	AggregateLookbackDays int
	Tracing               telemetry.TracingConfig
}

// Load reads and validates the configuration.
func Load() (Config, error) {
	env := config.New()
	cfg := Config{
		Env:             env.String("APP_ENV", "local"),
		Version:         env.String("APP_VERSION", "dev"),
		LogLevel:        env.String("LOG_LEVEL", "info"),
		LogFormat:       env.String("LOG_FORMAT", "json"),
		HTTP:            httpserver.Config{Addr: env.String("HTTP_ADDR", ":8080")},
		ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		ClickHouse: clickhouse.Config{
			URL:      env.String("CLICKHOUSE_URL", "http://localhost:8123"),
			Username: env.String("CLICKHOUSE_USERNAME", "icyre"),
			Password: env.String("CLICKHOUSE_PASSWORD", "icyre"),
			Database: env.String("CLICKHOUSE_DATABASE", "icyre"),
			Timeout:  env.Duration("CLICKHOUSE_TIMEOUT", 30*time.Second),
		},
		KafkaBrokers:          env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		MaxRetries:            env.Int("CONSUMER_MAX_RETRIES", 8),
		RetryBackoff:          env.Duration("CONSUMER_RETRY_BACKOFF", time.Second),
		BatchSize:             env.Int("ANALYTICS_BATCH_SIZE", 5000),
		BatchLinger:           env.Duration("ANALYTICS_BATCH_LINGER", 2*time.Second),
		CatalogURL:            env.String("CATALOG_URL", "http://localhost:8081"),
		CatalogTimeout:        env.Duration("CATALOG_TIMEOUT", 3*time.Second),
		TrackCacheTTL:         env.Duration("ANALYTICS_TRACK_CACHE_TTL", 10*time.Minute),
		AggregateEvery:        env.Duration("ANALYTICS_AGGREGATE_EVERY", time.Minute),
		AggregateLookbackDays: env.Int("ANALYTICS_AGGREGATE_LOOKBACK_DAYS", 1),
		Tracing: telemetry.TracingConfig{
			ServiceName:  ServiceName,
			Enabled:      env.Bool("OTEL_ENABLED", false),
			OTLPEndpoint: env.String("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
			Insecure:     env.Bool("OTEL_EXPORTER_OTLP_INSECURE", true),
			SampleRatio:  1,
		},
	}
	cfg.Tracing.Version, cfg.Tracing.Environment = cfg.Version, cfg.Env
	return cfg, env.Err()
}
