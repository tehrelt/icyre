// Command playback runs the Playback Service. This first slice accepts
// playback telemetry from the player and publishes playback.events;
// sessions and the server-side queue come later (EPIC-016).
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
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	httpadapter "github.com/tehrelt/icyre/services/playback/internal/adapters/http"
	kafkaadapter "github.com/tehrelt/icyre/services/playback/internal/adapters/kafka"
	"github.com/tehrelt/icyre/services/playback/internal/application"
	"github.com/tehrelt/icyre/services/playback/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "playback:", err)
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
	producer, err := platformkafka.NewProducer(platformkafka.ProducerConfig{Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName}, reg)
	if err != nil {
		return err
	}
	closers.Add("kafka producer", producer.Close)

	checks := health.New(0)
	checks.Add("redis", platformredis.Check(rdb))
	checks.Add("kafka", producer.Ping)

	// 5. Application.
	app := application.New(kafkaadapter.NewPublisher(producer, config.ServiceName))

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
	if err := srv.Run(serveCtx, cfg.ShutdownTimeout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	log.Info("playback service stopped")
	return nil
}
