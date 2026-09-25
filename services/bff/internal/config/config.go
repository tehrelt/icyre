// Package config loads Web BFF settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/services/bff/internal/application"
)

// ServiceName identifies the service in logs and traces.
const ServiceName = "web-bff"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	CatalogURL      string
	// LibraryURL enables per-listener marks (liked tracks); empty disables them.
	LibraryURL string
	// HistoryURL enables "Recently played" on Home; empty disables it.
	HistoryURL string
	// UpstreamTimeout bounds one call to an upstream service.
	UpstreamTimeout time.Duration
	Pages           application.Config
	Tracing         telemetry.TracingConfig
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
			RequestTimeout: env.Duration("HTTP_REQUEST_TIMEOUT", 5*time.Second),
		},
		ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		CatalogURL:      env.RequiredString("CATALOG_URL"),
		LibraryURL:      env.String("LIBRARY_URL", ""),
		HistoryURL:      env.String("HISTORY_URL", ""),
		UpstreamTimeout: env.Duration("UPSTREAM_TIMEOUT", 800*time.Millisecond),
		Pages: application.Config{
			PageBudget:    env.Duration("PAGE_BUDGET", 1500*time.Millisecond),
			GenreCacheTTL: env.Duration("GENRE_CACHE_TTL", 10*time.Minute),
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
