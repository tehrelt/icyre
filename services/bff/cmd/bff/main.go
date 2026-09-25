// Command bff runs the Web BFF: page-oriented aggregation for the web client.
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
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	catalogadapter "github.com/tehrelt/icyre/services/bff/internal/adapters/catalog"
	httpadapter "github.com/tehrelt/icyre/services/bff/internal/adapters/http"
	libraryadapter "github.com/tehrelt/icyre/services/bff/internal/adapters/library"
	"github.com/tehrelt/icyre/services/bff/internal/application"
	"github.com/tehrelt/icyre/services/bff/internal/config"
	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bff:", err)
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

	// 3. OpenTelemetry + Prometheus.
	stopTracing, err := telemetry.SetupTracing(ctx, cfg.Tracing)
	if err != nil {
		return err
	}
	closers.Add("tracing", stopTracing)
	reg := telemetry.NewRegistry()

	// 4. Upstream clients. The BFF owns no data: no database, no broker.
	catalog := catalogadapter.New(cfg.CatalogURL, httpclient.New(httpclient.Config{Timeout: cfg.UpstreamTimeout}), reg)

	// 5. Application.
	var library ports.Library
	if cfg.LibraryURL != "" {
		library = libraryadapter.New(cfg.LibraryURL, httpclient.New(httpclient.Config{Timeout: cfg.UpstreamTimeout}))
	}
	pages := application.New(catalog, library, cfg.Pages, log)

	// 6. Handlers. Readiness does not probe upstreams: pages degrade on
	// their own, and coupling readiness to Catalog would cascade outages.
	checks := health.New(0)
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(pages, log).Register(mux)

	handler := httpserver.Standard(mux, httpserver.StandardOptions{
		Service:        config.ServiceName,
		Log:            log,
		Metrics:        httpserver.NewHTTPMetrics(reg),
		RequestTimeout: cfg.HTTP.RequestTimeout,
	})

	// 7. Serve until SIGINT/SIGTERM.
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
	log.Info("web bff stopped")
	return nil
}
