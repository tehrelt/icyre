// Command catalog runs the Catalog Service.
//
//	catalog          serve the HTTP API
//	catalog migrate  apply database migrations and exit
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/outbox"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	httpadapter "github.com/tehrelt/icyre/services/catalog/internal/adapters/http"
	kafkaadapter "github.com/tehrelt/icyre/services/catalog/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/services/catalog/internal/adapters/postgres"
	"github.com/tehrelt/icyre/services/catalog/internal/application"
	"github.com/tehrelt/icyre/services/catalog/internal/config"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "catalog:", err)
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

	// 3. OpenTelemetry + Prometheus.
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

	checks := health.New(0)
	checks.Add("postgres", postgres.Check(pool))

	// Events go to catalog.outbox in the transaction of each change; the
	// relay moves them to Kafka (at-least-once end to end).
	var (
		publisher ports.EventPublisher = kafkaadapter.NopPublisher{}
		relay     *outbox.Relay
	)
	if cfg.Kafka.Enabled {
		producer, err := platformkafka.NewProducer(platformkafka.ProducerConfig{Brokers: cfg.Kafka.Brokers, ClientID: config.ServiceName}, reg)
		if err != nil {
			return err
		}
		closers.Add("kafka producer", producer.Close)
		checks.Add("kafka", producer.Ping)
		sink, err := outbox.NewSink(pool, pgadapter.OutboxTable)
		if err != nil {
			return err
		}
		publisher = kafkaadapter.NewPublisher(sink, config.ServiceName)
		if relay, err = outbox.NewRelay(pool, producer, outbox.RelayConfig{Table: pgadapter.OutboxTable}, log, reg); err != nil {
			return err
		}
	} else {
		log.Warn("kafka disabled: catalog events are not published")
	}

	// 5. Repositories and application.
	app := application.New(application.Deps{
		Artists:   pgadapter.NewArtistRepository(pool),
		Albums:    pgadapter.NewAlbumRepository(pool),
		Tracks:    pgadapter.NewTrackRepository(pool),
		Genres:    pgadapter.NewGenreRepository(pool),
		Publisher: publisher,
		Tx:        postgres.Transactor{Pool: pool},
		Log:       log,
	})

	// 6. Handlers.
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, log).Register(mux)

	handler := httpserver.Standard(mux, httpserver.StandardOptions{
		Service:        config.ServiceName,
		Log:            log,
		Metrics:        httpserver.NewHTTPMetrics(reg),
		RequestTimeout: cfg.HTTP.RequestTimeout,
	})

	// 7. Server, until SIGINT/SIGTERM. Readiness drains first so the load
	// balancer stops sending traffic before connections close.
	srv := httpserver.New(cfg.HTTP, handler, log)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		checks.Drain()
		cancelServe()
	}()
	relayDone := make(chan error, 1)
	if relay != nil {
		go func() { relayDone <- relay.Run(ctx) }()
	} else {
		relayDone <- nil
	}
	if err := srv.Run(serveCtx, cfg.ShutdownTimeout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	// Unsent rows stay in the outbox for the next start.
	if err := <-relayDone; err != nil && !errors.Is(err, context.Canceled) {
		log.Error("outbox relay stopped with error", logger.Err(err))
	}
	log.Info("catalog service stopped")
	return nil
}
