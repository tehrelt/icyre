// Package config loads Transcoder settings from the environment.
package config

import (
	"errors"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/config"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
)

// ServiceName identifies the worker.
const ServiceName = "transcoder"

// ConsumerGroup consumes media.events.
const ConsumerGroup = "transcoder"

// Config is the complete worker configuration.
type Config struct {
	Env             string
	Version         string
	LogLevel        string
	LogFormat       string
	HTTP            httpserver.Config // health and metrics only
	ShutdownTimeout time.Duration
	Storage         objectstore.Config
	Bucket          string
	KafkaBrokers    []string
	MaxRetries      int
	RetryBackoff    time.Duration
	FFmpegPath      string
	FFprobePath     string
	// JobTimeout bounds one track: download, transcode, upload.
	JobTimeout time.Duration
	// WorkDir holds per-job temp directories ("" = OS temp dir).
	WorkDir string
	Tracing telemetry.TracingConfig
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
		FFmpegPath:   env.String("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:  env.String("FFPROBE_PATH", "ffprobe"),
		JobTimeout:   env.Duration("TRANSCODE_TIMEOUT", 10*time.Minute),
		WorkDir:      env.String("TRANSCODE_WORK_DIR", ""),
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
	if cfg.JobTimeout <= 0 {
		return cfg, errors.New("TRANSCODE_TIMEOUT must be positive")
	}
	return cfg, nil
}
