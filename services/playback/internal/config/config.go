// Package config loads Playback Service settings from the environment.
package config

import (
	"fmt"
	"time"

	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service.
const ServiceName = "playback-service"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	Redis           redis.Config
	KafkaBrokers    []string
	JWKSURL         string
	Analytics       Analytics
	Tracing         telemetry.TracingConfig
}

// Analytics modes: where a playback event goes before the request returns.
const (
	// AnalyticsKafka publishes to playback.events; consumers (Analytics
	// Worker, Listening History) process it asynchronously. The default.
	AnalyticsKafka = "kafka"
	// AnalyticsSync writes to ClickHouse within the request (EPIC-038
	// experiment only: history and recommendations do not see the events).
	AnalyticsSync = "sync"
)

// Analytics settings.
type Analytics struct {
	Mode           string
	CatalogURL     string
	CatalogTimeout time.Duration
	ClickHouse     clickhouse.Config
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
			RequestTimeout: env.Duration("HTTP_REQUEST_TIMEOUT", 3*time.Second),
		},
		ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		Redis:           redis.Config{Addr: env.String("REDIS_ADDR", "localhost:6379"), Password: env.String("REDIS_PASSWORD", "")},
		KafkaBrokers:    env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		JWKSURL:         env.String("AUTH_JWKS_URL", "http://localhost:8083/api/v1/auth/.well-known/jwks.json"),
		Analytics: Analytics{
			Mode:           env.String("ANALYTICS_MODE", AnalyticsKafka),
			CatalogURL:     env.String("CATALOG_URL", "http://localhost:8081"),
			CatalogTimeout: env.Duration("CATALOG_TIMEOUT", 800*time.Millisecond),
			ClickHouse: clickhouse.Config{
				URL:      env.String("CLICKHOUSE_URL", "http://localhost:8123"),
				Username: env.String("CLICKHOUSE_USERNAME", "icyre"),
				Password: env.String("CLICKHOUSE_PASSWORD", "icyre"),
				Database: env.String("CLICKHOUSE_DATABASE", "icyre"),
				Timeout:  env.Duration("CLICKHOUSE_TIMEOUT", 2*time.Second),
			},
		},
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
	if m := cfg.Analytics.Mode; m != AnalyticsKafka && m != AnalyticsSync {
		return cfg, fmt.Errorf("ANALYTICS_MODE must be %q or %q, got %q", AnalyticsKafka, AnalyticsSync, m)
	}
	return cfg, nil
}
