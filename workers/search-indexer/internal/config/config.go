// Package config loads Search Indexer settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the worker.
const ServiceName = "search-indexer"

// ConsumerGroup consumes catalog.events.
const ConsumerGroup = "search-indexer"

// Config is the complete worker configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config // health and metrics only
	ShutdownTimeout time.Duration
	OpenSearch      opensearch.Config
	Shards          int
	Replicas        int
	KafkaBrokers    []string
	MaxRetries      int
	RetryBackoff    time.Duration
	CatalogURL      string
	CatalogTimeout  time.Duration
	Tracing         telemetry.TracingConfig
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
		OpenSearch: opensearch.Config{
			URL:      env.String("OPENSEARCH_URL", "http://localhost:9200"),
			Username: env.String("OPENSEARCH_USERNAME", ""),
			Password: env.String("OPENSEARCH_PASSWORD", ""),
			Timeout:  env.Duration("OPENSEARCH_TIMEOUT", 10*time.Second),
		},
		Shards:         env.Int("SEARCH_INDEX_SHARDS", 1),
		Replicas:       env.Int("SEARCH_INDEX_REPLICAS", 0),
		KafkaBrokers:   env.Strings("KAFKA_BROKERS", []string{"localhost:9094"}),
		MaxRetries:     env.Int("CONSUMER_MAX_RETRIES", 8),
		RetryBackoff:   env.Duration("CONSUMER_RETRY_BACKOFF", time.Second),
		CatalogURL:     env.String("CATALOG_URL", "http://localhost:8081"),
		CatalogTimeout: env.Duration("CATALOG_TIMEOUT", 3*time.Second),
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
