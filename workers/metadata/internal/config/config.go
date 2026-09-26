// Package config loads Metadata Worker settings from the environment.
package config

import (
	"errors"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the worker.
const ServiceName = "metadata-worker"

// ConsumerGroup consumes media.events.
const ConsumerGroup = "metadata-worker"

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
	Storage         objectstore.Config
	// Bucket is checked by the readiness probe; jobs name their own.
	Bucket       string
	KafkaBrokers []string
	MaxRetries   int
	RetryBackoff time.Duration
	FFprobePath  string
	// ProbeTimeout bounds one track; presigned URLs live as long.
	ProbeTimeout time.Duration
	Tracing      telemetry.TracingConfig
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
			StatementTimeout: env.Duration("DATABASE_STATEMENT_TIMEOUT", 5*time.Second),
			ApplicationName:  ServiceName,
		},
		MigrateOnStart: env.Bool("MIGRATE_ON_START", false),
		Storage: objectstore.Config{
			Endpoint:  env.String("S3_ENDPOINT", "localhost:9000"),
			Secure:    env.Bool("S3_SECURE", false),
			AccessKey: env.RequiredString("S3_ACCESS_KEY"),
			SecretKey: env.RequiredString("S3_SECRET_KEY"),
			Region:    env.String("S3_REGION", "us-east-1"),
			Timeout:   env.Duration("S3_TIMEOUT", 5*time.Second),
		},
		Bucket:       env.String("S3_MEDIA_BUCKET", media.Bucket),
		KafkaBrokers: env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		MaxRetries:   env.Int("CONSUMER_MAX_RETRIES", 5),
		RetryBackoff: env.Duration("CONSUMER_RETRY_BACKOFF", 2*time.Second),
		FFprobePath:  env.String("FFPROBE_PATH", "ffprobe"),
		ProbeTimeout: env.Duration("PROBE_TIMEOUT", time.Minute),
		Tracing: telemetry.TracingConfig{
			ServiceName:  ServiceName,
			Enabled:      env.Bool("OTEL_ENABLED", false),
			OTLPEndpoint: env.String("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
			Insecure:     env.Bool("OTEL_EXPORTER_OTLP_INSECURE", true),
			SampleRatio:  1,
		},
	}
	cfg.Tracing.Version, cfg.Tracing.Environment = cfg.Version, cfg.Env
	if err := env.Err(); err != nil {
		return cfg, err
	}
	if cfg.ProbeTimeout <= 0 {
		return cfg, errors.New("PROBE_TIMEOUT must be positive")
	}
	return cfg, nil
}
