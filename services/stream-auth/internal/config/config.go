// Package config loads Stream Authorization settings from the environment.
package config

import (
	"errors"
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the service.
const ServiceName = "stream-auth-service"

// Config is the complete service configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config
	ShutdownTimeout time.Duration
	Redis           redis.Config
	Storage         objectstore.Config
	Bucket          string
	CatalogURL      string
	CatalogTimeout  time.Duration
	JWKSURL         string
	// URLTTL is the lifetime of issued stream URLs (spec: 1–10 minutes).
	URLTTL         time.Duration
	StatusCacheTTL time.Duration
	Tracing        telemetry.TracingConfig
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
		Storage: objectstore.Config{
			Endpoint:       env.String("S3_ENDPOINT", "localhost:9000"),
			Secure:         env.Bool("S3_SECURE", false),
			PublicEndpoint: env.String("S3_PUBLIC_ENDPOINT", ""),
			PublicSecure:   env.Bool("S3_PUBLIC_SECURE", false),
			AccessKey:      env.RequiredString("S3_ACCESS_KEY"),
			SecretKey:      env.RequiredString("S3_SECRET_KEY"),
			Region:         env.String("S3_REGION", "us-east-1"),
			Timeout:        env.Duration("S3_TIMEOUT", 2*time.Second),
		},
		Bucket:         env.String("S3_MEDIA_BUCKET", "icyre-media"),
		CatalogURL:     env.String("CATALOG_URL", "http://localhost:8081"),
		CatalogTimeout: env.Duration("CATALOG_TIMEOUT", 800*time.Millisecond),
		JWKSURL:        env.String("AUTH_JWKS_URL", "http://localhost:8083/api/v1/auth/.well-known/jwks.json"),
		URLTTL:         env.Duration("STREAM_URL_TTL", 5*time.Minute),
		StatusCacheTTL: env.Duration("STREAM_STATUS_CACHE_TTL", 10*time.Second),
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
	if cfg.URLTTL < time.Minute || cfg.URLTTL > 10*time.Minute {
		return cfg, errors.New("STREAM_URL_TTL must be between 1m and 10m")
	}
	return cfg, nil
}
