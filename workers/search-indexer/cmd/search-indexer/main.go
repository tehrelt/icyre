// Command search-indexer keeps OpenSearch in step with Catalog and Playlist.
//
//	search-indexer          consume catalog.events and playlist.events, update the indices
//	search-indexer migrate  install index templates, create missing indices
//	search-indexer reindex  rebuild every index from Catalog and Playlist into new
//	                        versioned indices and switch the aliases
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/adapters/catalog"
	kafkaadapter "github.com/tehrelt/icyre/workers/search-indexer/internal/adapters/kafka"
	osadapter "github.com/tehrelt/icyre/workers/search-indexer/internal/adapters/opensearch"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/adapters/playlist"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/config"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/indices"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "search-indexer:", err)
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

	// 4. Adapters and application.
	osc := opensearch.New(cfg.OpenSearch, httpclient.New(httpclient.Config{Timeout: cfg.OpenSearch.Timeout}))
	idxOpts := indices.Options{Shards: cfg.Shards, Replicas: cfg.Replicas}
	cat := catalog.New(cfg.CatalogURL, httpclient.New(httpclient.Config{Timeout: cfg.CatalogTimeout}))
	sources := httpclient.New(httpclient.Config{Timeout: cfg.CatalogTimeout})
	app := application.New(cat, playlist.New(cfg.PlaylistURL, sources), playlist.NewProfiles(cfg.UserProfileURL, sources), osadapter.New(osc, false), log)

	switch {
	case len(args) > 0 && args[0] == "migrate":
		if err := indices.Ensure(ctx, osc, idxOpts); err != nil {
			return err
		}
		log.Info("search indices ready")
		return nil
	case len(args) > 0 && args[0] == "reindex":
		return reindex(ctx, osc, idxOpts, app, log)
	case len(args) > 0:
		return fmt.Errorf("unknown command %q", args[0])
	}

	if err := indices.Ensure(ctx, osc, idxOpts); err != nil {
		return err
	}

	// 5. Consumer: at-least-once; offsets are committed after the bulk write.
	consumer, err := platformkafka.NewConsumer(platformkafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName, Group: config.ConsumerGroup,
		Topics: []string{events.TopicCatalogEvents, events.TopicPlaylistEvents}, MaxRetries: cfg.MaxRetries, RetryBackoff: cfg.RetryBackoff, DLQ: true,
	}, kafkaadapter.Handler(app), log, platformkafka.NewConsumerMetrics(reg))
	if err != nil {
		return err
	}
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- consumer.Run(ctx) }()

	// 6. Health and metrics endpoint.
	checks := health.New(0)
	checks.Add("opensearch", osc.Check)
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
	log.Info("search indexer stopped")
	return nil
}

// reindex fills new versioned indices from Catalog and the Playlist Service
// and swaps the aliases.
// Readers keep the old indices until the swap; on failure nothing changes.
// Events consumed during the rebuild land in the old indices — run it with
// the consumer stopped, or run it again, for an exact result.
func reindex(ctx context.Context, osc *opensearch.Client, o indices.Options, app *application.Indexer, log *slog.Logger) error {
	if err := indices.Ensure(ctx, osc, o); err != nil {
		return err
	}
	start := time.Now()
	gen, err := indices.NewGeneration(ctx, osc)
	if err != nil {
		return err
	}
	stats, err := app.Rebuild(ctx, gen.Index)
	if err != nil {
		gen.Abandon(context.WithoutCancel(ctx), osc)
		return fmt.Errorf("rebuild: %w", err)
	}
	for _, index := range gen.Index {
		if err := osc.Refresh(ctx, index); err != nil {
			gen.Abandon(context.WithoutCancel(ctx), osc)
			return err
		}
	}
	if err := gen.Promote(ctx, osc); err != nil {
		return err
	}
	log.Info("reindex complete", "albums", stats.Albums, "tracks", stats.Tracks, "artists", stats.Artists, "playlists", stats.Playlists,
		"indices", fmt.Sprint(gen.Index), "duration", time.Since(start).Round(time.Millisecond).String())
	return nil
}
