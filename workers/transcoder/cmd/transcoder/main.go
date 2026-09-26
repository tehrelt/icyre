// Command transcoder turns uploaded masters into AAC variants: it consumes
// track.uploaded from media.events, writes tracks/{id}/audio/{64,128,256}.aac
// and publishes track.transcoded (specs/workers/transcoder.md).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/workers/transcoder/internal/adapters/ffmpeg"
	kafkaadapter "github.com/tehrelt/icyre/workers/transcoder/internal/adapters/kafka"
	"github.com/tehrelt/icyre/workers/transcoder/internal/adapters/storage"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
	"github.com/tehrelt/icyre/workers/transcoder/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "transcoder:", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Config.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// 2. Logger.
	log := logger.New(logger.Options{Service: config.ServiceName, Version: cfg.Version, Level: cfg.LogLevel, Format: cfg.LogFormat})
	slog.SetDefault(log)

	ctx, stop := shutdown.NotifyContext(context.Background())
	defer stop()
	closers := shutdown.NewStack(log)
	defer func() {
		cctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := closers.Close(cctx); err != nil {
			log.Error("shutdown finished with errors", logger.Err(err))
		}
	}()

	// 3. Telemetry.
	stopTracing, err := telemetry.SetupTracing(ctx, cfg.Tracing)
	if err != nil {
		return err
	}
	closers.Add("tracing", stopTracing)
	reg := telemetry.NewRegistry()

	// 4. Adapters and application.
	enc := ffmpeg.New(cfg.FFmpegPath, cfg.FFprobePath)
	if err := enc.Version(ctx); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	store, err := objectstore.New(cfg.Storage)
	if err != nil {
		return err
	}
	producer, err := platformkafka.NewProducer(platformkafka.ProducerConfig{Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName}, reg)
	if err != nil {
		return err
	}
	closers.Add("kafka producer", producer.Close)
	app := application.New(storage.New(store, cfg.Bucket), enc, kafkaadapter.NewPublisher(producer, config.ServiceName), cfg.WorkDir, log)

	// 5. Consumer: at-least-once; the offset is committed after the
	// variants are stored and track.transcoded is acknowledged.
	consumer, err := platformkafka.NewConsumer(platformkafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName, Group: config.ConsumerGroup,
		Topics: []string{events.TopicMediaEvents}, MaxRetries: cfg.MaxRetries, RetryBackoff: cfg.RetryBackoff, DLQ: true,
	}, kafkaadapter.Handler(app, kafkaadapter.NewMetrics(reg), cfg.JobTimeout), log, platformkafka.NewConsumerMetrics(reg))
	if err != nil {
		return err
	}
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- consumer.Run(ctx) }()

	// 6. Health and metrics endpoint.
	checks := health.New(0)
	checks.Add("object storage", store.Check(cfg.Bucket))
	checks.Add("kafka", producer.Ping)
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	srv := httpserver.New(cfg.HTTP, httpserver.Standard(mux, httpserver.StandardOptions{Service: config.ServiceName, Log: log, Metrics: httpserver.NewHTTPMetrics(reg)}), log)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		checks.Drain()
		cancelServe()
	}()
	if err := srv.Run(serveCtx, cfg.ShutdownTimeout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		log.Error("consumer stopped with error", logger.Err(err))
	}
	log.Info("transcoder stopped")
	return nil
}
