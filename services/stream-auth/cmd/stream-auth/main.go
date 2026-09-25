// Command stream-auth runs the Stream Authorization Service: it checks that a
// track may be played and returns a short-lived signed URL for its audio.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/services/stream-auth/internal/adapters/audit"
	"github.com/tehrelt/icyre/services/stream-auth/internal/adapters/catalog"
	httpadapter "github.com/tehrelt/icyre/services/stream-auth/internal/adapters/http"
	"github.com/tehrelt/icyre/services/stream-auth/internal/adapters/storage"
	"github.com/tehrelt/icyre/services/stream-auth/internal/application"
	"github.com/tehrelt/icyre/services/stream-auth/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stream-auth:", err)
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

	// 4. Connections.
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
	checks.Add("redis", platformredis.Check(rdb))
	checks.Add("object storage", store.Check(cfg.Bucket))

	// 5. Adapters.
	cacheMetrics := platformredis.NewCacheMetrics(reg)
	tracks := catalog.NewCachedTracks(
		catalog.New(cfg.CatalogURL, httpclient.New(httpclient.Config{Timeout: cfg.CatalogTimeout})),
		platformredis.NewCache[string](rdb, "track-status", cfg.StatusCacheTTL, log, cacheMetrics),
	)
	mediaStore := storage.New(store, cfg.Bucket)
	variants := storage.NewCachedVariants(mediaStore, platformredis.NewCache[[]media.Quality](rdb, "track-variants", time.Hour, log, cacheMetrics), log)

	// 6. Application.
	app := application.New(tracks, variants, mediaStore, audit.New(log, reg), cfg.URLTTL)

	// 7. Handlers.
	keys := authn.NewRemoteKeys(cfg.JWKSURL, httpclient.New(httpclient.Config{Timeout: 3 * time.Second}))
	verifier := authn.NewVerifier(keys, authn.NewRedisRevocations(rdb))
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, verifier).Register(mux)
	handler := httpserver.Standard(mux, httpserver.StandardOptions{
		Service: config.ServiceName, Log: log, Metrics: httpserver.NewHTTPMetrics(reg), RequestTimeout: cfg.HTTP.RequestTimeout,
	})

	// 8. Serve until SIGINT/SIGTERM.
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
	log.Info("stream authorization service stopped")
	return nil
}
