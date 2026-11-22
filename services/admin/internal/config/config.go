// Package config loads Admin Service settings from the environment.
package config

import (
	"errors"
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service in logs, traces, and metrics.
const ServiceName = "admin-service"

// ConsumerGroup consumes media.events to project transcode jobs.
const ConsumerGroup = "admin"

// Config is the complete service configuration.
type Config struct {
	Env               string
	Version           string
	LogLevel          string
	LogFormat         string
	HTTP              httpserver.Config
	ShutdownTimeout   time.Duration
	Postgres          postgres.Config
	MigrateOnStart    bool
	Redis             redis.Config
	Kafka             Kafka
	JWKSURL           string
	CatalogURL        string
	AuthURL           string
	MediaIngestURL    string
	UpstreamTimeout   time.Duration
	InternalToken     config.Secret
	ReconcileInterval time.Duration
	Tracing           telemetry.TracingConfig
}

// Local reports a developer or test environment.
func (c Config) Local() bool { return c.Env == "local" || c.Env == "test" }

// Kafka settings.
type Kafka struct {
	Enabled bool
	Brokers []string
}

// Load reads and validates the configuration from environment variables.
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
		Redis: redis.Config{
			Addr:     env.String("REDIS_ADDR", "localhost:6379"),
			Password: env.String("REDIS_PASSWORD", ""),
		},
		Kafka: Kafka{
			Enabled: env.Bool("KAFKA_ENABLED", true),
			Brokers: env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		},
		JWKSURL:           env.String("AUTH_JWKS_URL", "http://localhost:8083/api/v1/auth/.well-known/jwks.json"),
		CatalogURL:        env.String("CATALOG_URL", "http://localhost:8081"),
		AuthURL:           env.String("AUTH_URL", "http://localhost:8083"),
		MediaIngestURL:    env.String("MEDIA_INGEST_URL", "http://localhost:8084"),
		UpstreamTimeout:   env.Duration("UPSTREAM_TIMEOUT", 3*time.Second),
		ReconcileInterval: env.Duration("RECONCILE_INTERVAL", 15*time.Second),
		Tracing: telemetry.TracingConfig{
			ServiceName:  ServiceName,
			Enabled:      env.Bool("OTEL_ENABLED", false),
			OTLPEndpoint: env.String("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
			Insecure:     env.Bool("OTEL_EXPORTER_OTLP_INSECURE", true),
			SampleRatio:  1,
		},
	}
	if cfg.Local() {
		cfg.InternalToken = env.Secret("INTERNAL_SERVICE_TOKEN", "icyre-internal-dev-token")
	} else {
		cfg.InternalToken = env.RequiredSecret("INTERNAL_SERVICE_TOKEN")
	}
	cfg.Tracing.Version = cfg.Version
	cfg.Tracing.Environment = cfg.Env
	if err := env.Err(); err != nil {
		return cfg, err
	}
	if cfg.ReconcileInterval <= 0 {
		return cfg, errors.New("RECONCILE_INTERVAL must be positive")
	}
	return cfg, nil
}
