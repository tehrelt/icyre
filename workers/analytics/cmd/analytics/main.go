// Command analytics turns playback events into analytics in ClickHouse.
//
//	analytics                          consume playback.events into ClickHouse and
//	                                   recompute the recent daily aggregates periodically
//	analytics migrate                  apply the schema (libs/contracts/analytics)
//	analytics aggregate [-from] [-to]  recompute the daily aggregates of a day range (backfill)
//	analytics report [-from] [-to] [-limit]
//	                                   print plays, unique listeners, skips, completion rate
//	                                   and the most popular tracks and artists as JSON
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/workers/analytics/internal/adapters/catalog"
	chadapter "github.com/tehrelt/icyre/workers/analytics/internal/adapters/clickhouse"
	kafkaadapter "github.com/tehrelt/icyre/workers/analytics/internal/adapters/kafka"
	"github.com/tehrelt/icyre/workers/analytics/internal/application"
	"github.com/tehrelt/icyre/workers/analytics/internal/config"
)

// trackCacheSize bounds the Catalog cache (entries are ~100 bytes).
const trackCacheSize = 100_000

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "analytics:", err)
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

	ch := clickhouse.New(cfg.ClickHouse, httpclient.New(httpclient.Config{Timeout: cfg.ClickHouse.Timeout}))
	store := chadapter.New(ch)

	if len(args) > 0 {
		switch args[0] {
		case "migrate":
			return migrate(ctx, ch, log)
		case "aggregate":
			return aggregate(ctx, args[1:], store, log)
		case "report":
			return report(ctx, args[1:], store)
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	return serve(ctx, cfg, ch, store, log)
}

func serve(ctx context.Context, cfg config.Config, ch *clickhouse.Client, store *chadapter.Store, log *slog.Logger) error {
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

	// 4. Application.
	cat := application.NewTrackCache(
		catalog.New(cfg.CatalogURL, httpclient.New(httpclient.Config{Timeout: cfg.CatalogTimeout})),
		cfg.TrackCacheTTL, trackCacheSize)
	ingester := application.NewIngester(cat, store, log)
	aggregator := application.NewAggregator(store, cfg.AggregateLookbackDays, log, aggregationMetrics(reg))

	// 5. Consumer: batches are inserted, then their offsets committed.
	consumer, err := platformkafka.NewBatchConsumer(platformkafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName, Group: config.ConsumerGroup,
		Topics: []string{events.TopicPlaybackEvents}, MaxRetries: cfg.MaxRetries, RetryBackoff: cfg.RetryBackoff, DLQ: true,
	}, platformkafka.BatchConfig{MaxRecords: cfg.BatchSize, Linger: cfg.BatchLinger},
		kafkaadapter.Decode, ingester.Ingest, log, platformkafka.NewConsumerMetrics(reg))
	if err != nil {
		return err
	}
	// A consumer that gave up (sink down past all retries) stops the whole
	// process, so the orchestrator restarts it and the batch is redelivered.
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Run(runCtx)
		cancelRun()
	}()

	// 6. Aggregation jobs.
	aggDone := make(chan struct{})
	go func() {
		defer close(aggDone)
		aggregator.Run(runCtx, cfg.AggregateEvery)
	}()

	// 7. Health and metrics endpoint.
	checks := health.New(0)
	checks.Add("clickhouse", ch.Check)
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	srv := httpserver.New(cfg.HTTP, httpserver.Standard(mux, httpserver.StandardOptions{Service: config.ServiceName, Log: log, Metrics: httpserver.NewHTTPMetrics(reg)}), log)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	go func() {
		<-runCtx.Done()
		checks.Drain()
		cancelServe()
	}()
	if err := srv.Run(serveCtx, cfg.ShutdownTimeout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	<-aggDone
	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("consumer: %w", err)
	}
	log.Info("analytics worker stopped")
	return nil
}

// aggregationMetrics registers analytics_aggregation_* and returns the
// observer the Aggregator reports each run to.
func aggregationMetrics(reg prometheus.Registerer) func(error, time.Duration) {
	runs := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_aggregation_runs_total",
		Help: "Aggregation job runs by result (ok, error).",
	}, []string{"result"})
	took := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "analytics_aggregation_duration_seconds",
		Help:    "Time to recompute the recent days' aggregates.",
		Buckets: prometheus.DefBuckets,
	})
	reg.MustRegister(runs, took)
	return func(err error, d time.Duration) {
		result := "ok"
		if err != nil {
			result = "error"
		}
		runs.WithLabelValues(result).Inc()
		took.Observe(d.Seconds())
	}
}

func migrate(ctx context.Context, ch *clickhouse.Client, log *slog.Logger) error {
	ms, err := clickhouse.LoadMigrations(analytics.Migrations, analytics.MigrationsDir)
	if err != nil {
		return err
	}
	if err := ch.Check(ctx); err != nil {
		return err
	}
	n, err := ch.Migrate(ctx, ms)
	if err != nil {
		return err
	}
	log.Info("clickhouse schema ready", "applied", n, "latest", ms[len(ms)-1].Version)
	return nil
}

// dayRange parses -from and -to (YYYY-MM-DD, UTC); both default to today.
func dayRange(name string, args []string, extra func(*flag.FlagSet)) (from, to time.Time, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	today := time.Now().UTC().Format(time.DateOnly)
	fromS := fs.String("from", today, "first day, YYYY-MM-DD (UTC)")
	toS := fs.String("to", today, "last day, YYYY-MM-DD (UTC)")
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(args); err != nil {
		return from, to, err
	}
	if from, err = time.Parse(time.DateOnly, *fromS); err != nil {
		return from, to, fmt.Errorf("-from: %w", err)
	}
	if to, err = time.Parse(time.DateOnly, *toS); err != nil {
		return from, to, fmt.Errorf("-to: %w", err)
	}
	if to.Before(from) {
		return from, to, errors.New("-to is before -from")
	}
	return from, to, nil
}

func aggregate(ctx context.Context, args []string, store *chadapter.Store, log *slog.Logger) error {
	from, to, err := dayRange("aggregate", args, nil)
	if err != nil {
		return err
	}
	start := time.Now()
	if err := application.NewAggregator(store, 0, log, nil).Recompute(ctx, from, to); err != nil {
		return err
	}
	log.Info("aggregates recomputed", "from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly),
		"took", time.Since(start).Round(time.Millisecond).String())
	return nil
}

func report(ctx context.Context, args []string, store *chadapter.Store) error {
	var limit int
	from, to, err := dayRange("report", args, func(fs *flag.FlagSet) {
		fs.IntVar(&limit, "limit", 10, "tracks and artists in the charts")
	})
	if err != nil {
		return err
	}
	if limit < 1 || limit > 1000 {
		return errors.New("-limit must be within 1..1000")
	}
	rep, err := store.Report(ctx, from, to, limit)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
