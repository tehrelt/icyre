// Package config loads Recommendation Service settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service.
const ServiceName = "recommendation-service"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	Redis           redis.Config
	// JWKSURL is where Auth publishes its signing keys.
	JWKSURL string
	Tracing telemetry.TracingConfig
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
		Redis:           redis.Config{Addr: env.String("REDIS_ADDR", "localhost:6379"), Password: env.String("REDIS_PASSWORD", "")},
		JWKSURL:         env.String("AUTH_JWKS_URL", "http://localhost:8083/api/v1/auth/.well-known/jwks.json"),
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
