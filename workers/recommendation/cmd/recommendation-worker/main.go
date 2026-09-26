// Command recommendation-worker builds personal recommendations
// (specs/workers/recommendation-worker.md): it keeps likes (library.events)
// and audio features (media.events), reads listening history and
// popularity from ClickHouse and the catalog from Catalog, scores the
// catalog for every user and publishes ready sets to Redis.
//
//	recommendation-worker          consume signals and rebuild the sets periodically
//	recommendation-worker migrate  apply database migrations and exit
//	recommendation-worker build    build the sets once and exit
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/workers/recommendation/internal/adapters/catalog"
	chadapter "github.com/tehrelt/icyre/workers/recommendation/internal/adapters/clickhouse"
	kafkaadapter "github.com/tehrelt/icyre/workers/recommendation/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/workers/recommendation/internal/adapters/postgres"
	redisadapter "github.com/tehrelt/icyre/workers/recommendation/internal/adapters/redis"
	"github.com/tehrelt/icyre/workers/recommendation/internal/application"
	"github.com/tehrelt/icyre/workers/recommendation/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "recommendation-worker:", err)
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
	if len(args) > 0 && args[0] != "build" {
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
	ch := clickhouse.New(cfg.ClickHouse, httpclient.New(httpclient.Config{Timeout: cfg.ClickHouse.Timeout}))

	// 5. Application.
	repo := pgadapter.New(pool)
	builder := application.NewBuilder(
		catalog.New(cfg.CatalogURL, httpclient.New(httpclient.Config{Timeout: cfg.CatalogTimeout})),
		chadapter.New(ch), repo, redisadapter.New(rdb, cfg.SetTTL),
		application.Options{HistoryDays: cfg.HistoryDays, PopularityDays: cfg.PopularityDays, Tracks: cfg.Tracks, Artists: cfg.Artists},
		log)
	if len(args) > 0 { // build
		st, err := builder.Build(ctx)
		if err != nil {
			return err
		}
		log.Info("recommendations built", "users", st.Users, "candidates", st.Candidates, "took", st.Took.Round(time.Millisecond).String())
		return nil
	}

	// 6. Consumer: at-least-once; likes compare timestamps and features keep
	// the latest master, so redelivery is harmless.
	consumer, err := platformkafka.NewConsumer(platformkafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName, Group: config.ConsumerGroup,
		Topics: []string{events.TopicLibraryEvents, events.TopicMediaEvents}, MaxRetries: cfg.MaxRetries, RetryBackoff: cfg.RetryBackoff, DLQ: true,
	}, kafkaadapter.Handler(repo), log, platformkafka.NewConsumerMetrics(reg))
	if err != nil {
		return err
	}
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- consumer.Run(ctx) }()

	// 7. Periodic build.
	buildDone := make(chan struct{})
	go func() {
		defer close(buildDone)
		builder.Run(ctx, cfg.BuildEvery, buildMetrics(reg))
	}()

	// 8. Health and metrics endpoint.
	checks := health.New(0)
	checks.Add("postgres", postgres.Check(pool))
	checks.Add("redis", platformredis.Check(rdb))
	checks.Add("clickhouse", ch.Check)
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
	<-buildDone
	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		log.Error("consumer stopped with error", logger.Err(err))
	}
	log.Info("recommendation worker stopped")
	return nil
}

// buildMetrics registers recommendation_build_* and returns the observer
// of each build.
func buildMetrics(reg prometheus.Registerer) func(application.Stats, error) {
	runs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "recommendation_build_runs_total",
		Help: "Recommendation builds by result (ok, error).",
	}, []string{"result"})
	took := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "recommendation_build_duration_seconds",
		Help:    "Time to build every recommendation set.",
		Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
	})
	users := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "recommendation_personal_sets",
		Help: "Users with a personal set in the last successful build.",
	})
	reg.MustRegister(runs, took, users)
	return func(st application.Stats, err error) {
		if err != nil {
			runs.WithLabelValues("error").Inc()
			return
		}
		runs.WithLabelValues("ok").Inc()
		took.Observe(st.Took.Seconds())
		users.Set(float64(st.Users))
	}
}
