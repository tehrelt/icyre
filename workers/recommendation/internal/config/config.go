// Package config loads Recommendation Worker settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the worker.
const ServiceName = "recommendation-worker"

// ConsumerGroup consumes library.events and media.events.
const ConsumerGroup = "recommendation"

// Config is the complete worker configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config // health and metrics only
	ShutdownTimeout time.Duration
	Postgres        postgres.Config
	MigrateOnStart  bool
	Redis           redis.Config
	ClickHouse      clickhouse.Config
	KafkaBrokers    []string
	MaxRetries      int
	RetryBackoff    time.Duration
	CatalogURL      string
	CatalogTimeout  time.Duration
	// BuildEvery is the period of the build; SetTTL how long a published set
	// lives (several periods, so a failed build does not empty the home page).
	BuildEvery     time.Duration
	SetTTL         time.Duration
	HistoryDays    int
	PopularityDays int
	Tracks         int
	Artists        int
	Tracing        telemetry.TracingConfig
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
		Postgres: postgres.Config{
			DSN:              env.RequiredString("DATABASE_URL"),
			MaxConns:         int32(env.Int("DATABASE_MAX_CONNS", 5)),
			ConnectTimeout:   env.Duration("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
			StatementTimeout: env.Duration("DATABASE_STATEMENT_TIMEOUT", 30*time.Second),
			ApplicationName:  ServiceName,
		},
		MigrateOnStart: env.Bool("MIGRATE_ON_START", false),
		Redis:          redis.Config{Addr: env.String("REDIS_ADDR", "localhost:6379"), Password: env.String("REDIS_PASSWORD", "")},
		ClickHouse: clickhouse.Config{
			URL:      env.String("CLICKHOUSE_URL", "http://localhost:8123"),
			Username: env.String("CLICKHOUSE_USERNAME", "icyre"),
			Password: env.String("CLICKHOUSE_PASSWORD", "icyre"),
			Database: env.String("CLICKHOUSE_DATABASE", "icyre"),
			Timeout:  env.Duration("CLICKHOUSE_TIMEOUT", 60*time.Second),
		},
		KafkaBrokers:   env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		MaxRetries:     env.Int("CONSUMER_MAX_RETRIES", 8),
		RetryBackoff:   env.Duration("CONSUMER_RETRY_BACKOFF", time.Second),
		CatalogURL:     env.String("CATALOG_URL", "http://localhost:8081"),
		CatalogTimeout: env.Duration("CATALOG_TIMEOUT", 5*time.Second),
		BuildEvery:     env.Duration("RECOMMENDATION_BUILD_EVERY", 10*time.Minute),
		SetTTL:         env.Duration("RECOMMENDATION_SET_TTL", 24*time.Hour),
		HistoryDays:    env.Int("RECOMMENDATION_HISTORY_DAYS", 90),
		PopularityDays: env.Int("RECOMMENDATION_POPULARITY_DAYS", 30),
		Tracks:         env.Int("RECOMMENDATION_TRACKS", 50),
		Artists:        env.Int("RECOMMENDATION_ARTISTS", 20),
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
