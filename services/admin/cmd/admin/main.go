// Command admin runs the Admin Service.
//
//	admin          serve the HTTP API, Kafka consumer, and pending reconciler
//	admin migrate  apply database migrations and exit
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/services/admin/internal/adapters/clients"
	httpadapter "github.com/tehrelt/icyre/services/admin/internal/adapters/http"
	kafkaadapter "github.com/tehrelt/icyre/services/admin/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/services/admin/internal/adapters/postgres"
	"github.com/tehrelt/icyre/services/admin/internal/application"
	"github.com/tehrelt/icyre/services/admin/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logger.New(logger.Options{Service: config.ServiceName, Version: cfg.Version, Level: cfg.LogLevel, Format: cfg.LogFormat})
	slog.SetDefault(log)

	ctx, stop := shutdown.NotifyContext(context.Background())
	defer stop()
	closers := shutdown.NewStack(log)
	defer closeStack(closers, cfg.ShutdownTimeout, log)

	stopTracing, err := telemetry.SetupTracing(ctx, cfg.Tracing)
	if err != nil {
		return err
	}
	closers.Add("tracing", stopTracing)
	reg := telemetry.NewRegistry()

	pool, err := postgres.Open(ctx, cfg.Postgres, reg)
	if err != nil {
		return err
	}
	closers.AddFunc("postgres", pool.Close)

	if handled, err := handleMigrateArgs(ctx, args, pool, cfg.MigrateOnStart, log); handled || err != nil {
		return err
	}

	rdb, err := platformredis.Open(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	closers.Add("redis", func(context.Context) error { return rdb.Close() })

	checks := health.New(0)
	checks.Add("postgres", postgres.Check(pool))
	checks.Add("redis", platformredis.Check(rdb))

	app := buildApp(pool, cfg, log)
	consumerDone, err := startConsumer(ctx, cfg, app, log, reg)
	if err != nil {
		return err
	}
	reconcileDone := startReconciler(ctx, app, cfg.ReconcileInterval, log)

	handler := buildHTTPHandler(cfg, app, rdb, checks, reg, log)
	srv := httpserver.New(cfg.HTTP, handler, log)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		checks.Drain()
		cancelServe()
	}()

	if err := srv.Run(serveCtx, cfg.ShutdownTimeout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	<-reconcileDone
	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		log.Error("consumer stopped with error", logger.Err(err))
	}
	log.Info("admin service stopped")
	return nil
}

func closeStack(closers *shutdown.Stack, timeout time.Duration, log *slog.Logger) {
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := closers.Close(cctx); err != nil {
		log.Error("shutdown finished with errors", logger.Err(err))
	}
}

func handleMigrateArgs(ctx context.Context, args []string, pool *pgxpool.Pool, migrateOnStart bool, log *slog.Logger) (bool, error) {
	if len(args) > 0 && args[0] == "migrate" {
		return true, postgres.Migrate(ctx, pool, pgadapter.Schema, pgadapter.Migrations(), log)
	}
	if len(args) > 0 {
		return true, fmt.Errorf("unknown command %q", args[0])
	}
	if migrateOnStart {
		if err := postgres.Migrate(ctx, pool, pgadapter.Schema, pgadapter.Migrations(), log); err != nil {
			return false, err
		}
	}
	return false, nil
}

func buildApp(pool *pgxpool.Pool, cfg config.Config, log *slog.Logger) *application.Service {
	repo := pgadapter.NewRepository(pool)
	token := cfg.InternalToken.Value()
	catClient := clients.NewCatalogClient(cfg.CatalogURL, token, httpclient.New(httpclient.Config{Timeout: cfg.UpstreamTimeout}))
	authClient := clients.NewAuthClient(cfg.AuthURL, token, httpclient.New(httpclient.Config{Timeout: cfg.UpstreamTimeout}))
	ingestClient := clients.NewMediaIngestClient(cfg.MediaIngestURL, token, httpclient.New(httpclient.Config{Timeout: cfg.UpstreamTimeout}))

	return application.New(application.Deps{
		Ops:         repo,
		Audit:       repo,
		Jobs:        repo,
		Catalog:     catClient,
		Auth:        authClient,
		MediaIngest: ingestClient,
		Tx:          postgres.Transactor{Pool: pool},
		Log:         log,
	})
}

func startConsumer(ctx context.Context, cfg config.Config, app *application.Service, log *slog.Logger, reg prometheus.Registerer) (<-chan error, error) {
	done := make(chan error, 1)
	if !cfg.Kafka.Enabled {
		log.Warn("kafka disabled: transcode job projections are not updated")
		close(done)
		return done, nil
	}
	consumer, err := platformkafka.NewConsumer(platformkafka.ConsumerConfig{
		Brokers:      cfg.Kafka.Brokers,
		ClientID:     config.ServiceName,
		Group:        config.ConsumerGroup,
		Topics:       []string{events.TopicMediaEvents},
		MaxRetries:   5,
		RetryBackoff: 500 * time.Millisecond,
		DLQ:          true,
	}, kafkaadapter.MediaEventsHandler(app, log), log, platformkafka.NewConsumerMetrics(reg))
	if err != nil {
		return nil, err
	}
	go func() { done <- consumer.Run(ctx) }()
	return done, nil
}

func startReconciler(ctx context.Context, app *application.Service, interval time.Duration, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := app.ReconcilePending(ctx, application.DefaultPageSize); err != nil && !errors.Is(err, context.Canceled) {
					log.WarnContext(ctx, "pending operation reconciliation failed", logger.Err(err))
				}
			}
		}
	}()
	return done
}

func buildHTTPHandler(cfg config.Config, app *application.Service, rdb *platformredis.Client, checks *health.Checker, reg *prometheus.Registry, log *slog.Logger) http.Handler {
	keys := authn.NewRemoteKeys(cfg.JWKSURL, httpclient.New(httpclient.Config{Timeout: 3 * time.Second}))
	verifier := authn.NewVerifier(keys, authn.NewRedisRevocations(rdb))

	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, verifier, log).Register(mux)

	return httpserver.Standard(mux, httpserver.StandardOptions{
		Service:        config.ServiceName,
		Log:            log,
		Metrics:        httpserver.NewHTTPMetrics(reg),
		RequestTimeout: cfg.HTTP.RequestTimeout,
	})
}
