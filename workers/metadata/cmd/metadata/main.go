// Command metadata extracts technical metadata of uploaded masters: it
// consumes track.uploaded from media.events, probes the master with
// ffprobe, stores the result and publishes media.metadata_extracted
// through its outbox (specs/workers/metadata.md).
//
//	metadata          consume media.events
//	metadata migrate  apply database migrations and exit
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
	"github.com/tehrelt/icyre/libs/platform/outbox"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/workers/metadata/internal/adapters/ffprobe"
	kafkaadapter "github.com/tehrelt/icyre/workers/metadata/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/workers/metadata/internal/adapters/postgres"
	"github.com/tehrelt/icyre/workers/metadata/internal/adapters/storage"
	"github.com/tehrelt/icyre/workers/metadata/internal/application"
	"github.com/tehrelt/icyre/workers/metadata/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "metadata:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
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

	// 4. Connections.
	pool, err := postgres.Open(ctx, cfg.Postgres, reg)
	if err != nil {
		return err
	}
	closers.AddFunc("postgres", pool.Close)
	if len(args) > 0 && args[0] == "migrate" {
		return postgres.Migrate(ctx, pool, pgadapter.Schema, pgadapter.Migrations(), log)
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q", args[0])
	}
	if cfg.MigrateOnStart {
		if err := postgres.Migrate(ctx, pool, pgadapter.Schema, pgadapter.Migrations(), log); err != nil {
			return err
		}
	}
	prober := ffprobe.New(cfg.FFprobePath)
	if err := prober.Version(ctx); err != nil {
		return fmt.Errorf("ffprobe: %w", err)
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
	relay, err := outbox.NewRelay(pool, producer, outbox.RelayConfig{Table: pgadapter.OutboxTable}, log, reg)
	if err != nil {
		return err
	}

	// 5. Application.
	repo := pgadapter.New(pool, kafkaadapter.Encoder(config.ServiceName))
	app := application.New(storage.New(store, cfg.ProbeTimeout), prober, repo, log)

	// 6. Consumer: at-least-once; the offset is committed after the row and
	// its event are stored, the upload ID makes redelivery harmless.
	consumer, err := platformkafka.NewConsumer(platformkafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName, Group: config.ConsumerGroup,
		Topics: []string{events.TopicMediaEvents}, MaxRetries: cfg.MaxRetries, RetryBackoff: cfg.RetryBackoff, DLQ: true,
	}, kafkaadapter.Handler(app, kafkaadapter.NewMetrics(reg), cfg.ProbeTimeout), log, platformkafka.NewConsumerMetrics(reg))
	if err != nil {
		return err
	}
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- consumer.Run(ctx) }()
	waitRelay := relay.Start(ctx)

	// 7. Health and metrics endpoint.
	checks := health.New(0)
	checks.Add("postgres", postgres.Check(pool))
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
	if err := waitRelay(); err != nil {
		log.Error("outbox relay stopped with error", logger.Err(err))
	}
	log.Info("metadata worker stopped")
	return nil
}
