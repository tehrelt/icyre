// Command search runs the Search Service: full-text search and
// autocomplete over the OpenSearch indices kept by the Search Indexer.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	httpadapter "github.com/tehrelt/icyre/services/search/internal/adapters/http"
	osadapter "github.com/tehrelt/icyre/services/search/internal/adapters/opensearch"
	"github.com/tehrelt/icyre/services/search/internal/application"
	"github.com/tehrelt/icyre/services/search/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "search:", err)
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
	osc := opensearch.New(cfg.OpenSearch, httpclient.New(httpclient.Config{Timeout: cfg.OpenSearch.Timeout}))
	app := application.New(osadapter.New(osc))

	// 5. Handlers.
	checks := health.New(0)
	checks.Add("opensearch", osc.Check)
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, log).Register(mux)
	handler := httpserver.Standard(mux, httpserver.StandardOptions{
		Service: config.ServiceName, Log: log, Metrics: httpserver.NewHTTPMetrics(reg), RequestTimeout: cfg.HTTP.RequestTimeout,
	})

	// 6. Serve until SIGINT/SIGTERM.
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
	log.Info("search service stopped")
	return nil
}
