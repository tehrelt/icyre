// Package config loads Analytics Worker settings from the environment.
package config

import (
	"time"

	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/config"
)

// ServiceName identifies the worker.
const ServiceName = "analytics"

// Config is the complete worker configuration.
type Config struct {
	Version    string
	LogLevel   string
	LogFormat  string
	ClickHouse clickhouse.Config
}

// Load reads and validates the configuration.
func Load() (Config, error) {
	env := config.New()
	cfg := Config{
		Version:   env.String("APP_VERSION", "dev"),
		LogLevel:  env.String("LOG_LEVEL", "info"),
		LogFormat: env.String("LOG_FORMAT", "json"),
		ClickHouse: clickhouse.Config{
			URL:      env.String("CLICKHOUSE_URL", "http://localhost:8123"),
			Username: env.String("CLICKHOUSE_USERNAME", "icyre"),
			Password: env.String("CLICKHOUSE_PASSWORD", "icyre"),
			Database: env.String("CLICKHOUSE_DATABASE", "icyre"),
			Timeout:  env.Duration("CLICKHOUSE_TIMEOUT", 30*time.Second),
		},
	}
	return cfg, env.Err()
}
