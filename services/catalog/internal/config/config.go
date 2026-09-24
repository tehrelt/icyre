// Package config loads Catalog Service settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service in logs, traces and event envelopes.
const ServiceName = "catalog-service"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	Postgres        postgres.Config
	// MigrateOnStart applies migrations before serving (handy locally).
	MigrateOnStart bool
	Kafka          Kafka
	Tracing        telemetry.TracingConfig
}

// Kafka settings.
type Kafka struct {
	Enabled bool
	Brokers []string
}

// Load reads and validates the configuration.
func Load() (Config, error) {
	env := config.New()
	cfg := Config{
		Env:       env.String("APP_ENV", "local"),
		Version:   env.String("APP_VERSION", "dev"),
		LogLevel:  env.String("LOG_LEVEL", "info"),
		LogFormat: env.String("LOG_FORMAT", "json"),
		HTTP: httpserver.Config{
			Addr:           env.String("HTTP_ADDR", ":8080"),
			RequestTimeout: env.Duration("HTTP_REQUEST_TIMEOUT", 10*time.Second),
		},
		ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		Postgres: postgres.Config{
			DSN:              env.RequiredString("DATABASE_URL"),
			MaxConns:         int32(env.Int("DATABASE_MAX_CONNS", 10)),
			MinConns:         int32(env.Int("DATABASE_MIN_CONNS", 1)),
			MaxConnLifetime:  env.Duration("DATABASE_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:  env.Duration("DATABASE_MAX_CONN_IDLE_TIME", 10*time.Minute),
			ConnectTimeout:   env.Duration("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
			StatementTimeout: env.Duration("DATABASE_STATEMENT_TIMEOUT", 5*time.Second),
			ApplicationName:  ServiceName,
		},
		MigrateOnStart: env.Bool("MIGRATE_ON_START", false),
		Kafka: Kafka{
			Enabled: env.Bool("KAFKA_ENABLED", true),
			Brokers: env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		},
		Tracing: telemetry.TracingConfig{
			ServiceName:  ServiceName,
			Enabled:      env.Bool("OTEL_ENABLED", false),
			OTLPEndpoint: env.String("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
			Insecure:     env.Bool("OTEL_EXPORTER_OTLP_INSECURE", true),
			SampleRatio:  1,
		},
	}
	cfg.Tracing.Version = cfg.Version
	cfg.Tracing.Environment = cfg.Env
	return cfg, env.Err()
}
