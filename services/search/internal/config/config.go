// Package config loads Search Service settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service.
const ServiceName = "search-service"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	OpenSearch      opensearch.Config
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
			RequestTimeout: env.Duration("HTTP_REQUEST_TIMEOUT", 3*time.Second),
		},
		ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		OpenSearch: opensearch.Config{
			URL:      env.String("OPENSEARCH_URL", "http://localhost:9200"),
			Username: env.String("OPENSEARCH_USERNAME", ""),
			Password: env.String("OPENSEARCH_PASSWORD", ""),
			// The spec's latency target for search is under a second.
			Timeout: env.Duration("OPENSEARCH_TIMEOUT", time.Second),
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
	return cfg, env.Err()
}
