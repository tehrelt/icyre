// Package config loads Playlist settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service.
const ServiceName = "playlist-service"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	Postgres        postgres.Config
	MigrateOnStart  bool
	Redis           redis.Config
	KafkaEnabled    bool
	KafkaBrokers    []string
	Tracing         telemetry.TracingConfig
	// JWKSURL is where Auth publishes its signing keys.
	JWKSURL        string
	CatalogURL     string
	CatalogTimeout time.Duration
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
			ConnectTimeout:   env.Duration("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
			StatementTimeout: env.Duration("DATABASE_STATEMENT_TIMEOUT", 5*time.Second),
			ApplicationName:  ServiceName,
		},
		MigrateOnStart: env.Bool("MIGRATE_ON_START", false),
		Redis:          redis.Config{Addr: env.String("REDIS_ADDR", "localhost:6379"), Password: env.String("REDIS_PASSWORD", "")},
		KafkaEnabled:   env.Bool("KAFKA_ENABLED", true),
		KafkaBrokers:   env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		JWKSURL:        env.String("AUTH_JWKS_URL", "http://localhost:8083/api/v1/auth/.well-known/jwks.json"),
		CatalogURL:     env.String("CATALOG_URL", "http://localhost:8081"),
		CatalogTimeout: env.Duration("CATALOG_TIMEOUT", 800*time.Millisecond),
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
