// Command media-ingest runs the Media Ingest Service.
//
//	media-ingest          serve the HTTP API
//	media-ingest migrate  apply database migrations and exit
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/libs/platform/outbox"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/services/media-ingest/internal/adapters/catalog"
	httpadapter "github.com/tehrelt/icyre/services/media-ingest/internal/adapters/http"
	kafkaadapter "github.com/tehrelt/icyre/services/media-ingest/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/services/media-ingest/internal/adapters/postgres"
	"github.com/tehrelt/icyre/services/media-ingest/internal/adapters/storage"
	"github.com/tehrelt/icyre/services/media-ingest/internal/application"
	"github.com/tehrelt/icyre/services/media-ingest/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "media-ingest:", err)
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

	rdb, err := platformredis.Open(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	closers.Add("redis", func(context.Context) error { return rdb.Close() })

	store, err := objectstore.New(cfg.Storage)
	if err != nil {
		return err
	}

	checks := health.New(0)
	checks.Add("postgres", postgres.Check(pool))
	checks.Add("redis", platformredis.Check(rdb))
	checks.Add("storage", store.Check(cfg.Bucket))

	// Settled uploads go to media.outbox in the transaction of the status
	// change; the relay moves them to Kafka (at-least-once end to end).
	var (
		publisher application.Publisher = kafkaadapter.NopPublisher{}
		relay     *outbox.Relay
	)
	if cfg.KafkaEnabled {
		producer, err := platformkafka.NewProducer(platformkafka.ProducerConfig{Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName}, reg)
		if err != nil {
			return err
		}
		closers.Add("kafka producer", producer.Close)
		checks.Add("kafka", producer.Ping)
		sink, err := outbox.NewSink(pool, pgadapter.OutboxTable)
		if err != nil {
			return err
		}
		publisher = kafkaadapter.NewPublisher(sink, config.ServiceName, cfg.Bucket)
		if relay, err = outbox.NewRelay(pool, producer, outbox.RelayConfig{Table: pgadapter.OutboxTable}, log, reg); err != nil {
			return err
		}
	}

	// 5. Application. Uploads are checked against Catalog.
	cat := catalog.New(cfg.CatalogURL, httpclient.New(httpclient.Config{Timeout: cfg.CatalogTimeout}))
	limits := application.Limits{MaxSize: cfg.MaxUploadBytes, URLTTL: cfg.UploadURLTTL}
	app := application.New(pgadapter.New(pool), cat, storage.New(store, cfg.Bucket), publisher, postgres.Transactor{Pool: pool}, limits, log)

	// 6. Handlers. Access tokens are verified against Auth's JWKS.
	keys := authn.NewRemoteKeys(cfg.JWKSURL, httpclient.New(httpclient.Config{Timeout: 3 * time.Second}))
	verifier := authn.NewVerifier(keys, authn.NewRedisRevocations(rdb))
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, verifier, log).Register(mux)
	handler := httpserver.Standard(mux, httpserver.StandardOptions{
		Service: config.ServiceName, Log: log, Metrics: httpserver.NewHTTPMetrics(reg), RequestTimeout: cfg.HTTP.RequestTimeout,
	})

	// 7. Serve until SIGINT/SIGTERM.
	srv := httpserver.New(cfg.HTTP, handler, log)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		checks.Drain()
		cancelServe()
	}()
	waitRelay := func() error { return nil }
	if relay != nil {
		waitRelay = relay.Start(ctx)
	}
	if err := srv.Run(serveCtx, cfg.ShutdownTimeout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if err := waitRelay(); err != nil {
		log.Error("outbox relay stopped with error", logger.Err(err))
	}
	log.Info("media ingest service stopped")
	return nil
}
