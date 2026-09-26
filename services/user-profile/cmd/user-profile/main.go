// Command user-profile runs the User Profile Service.
//
//	user-profile          serve the HTTP API and consume auth.events
//	user-profile migrate  apply database migrations and exit
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
	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/health"
	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/outbox"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	httpadapter "github.com/tehrelt/icyre/services/user-profile/internal/adapters/http"
	kafkaadapter "github.com/tehrelt/icyre/services/user-profile/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/services/user-profile/internal/adapters/postgres"
	"github.com/tehrelt/icyre/services/user-profile/internal/application"
	"github.com/tehrelt/icyre/services/user-profile/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "user-profile:", err)
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

	checks := health.New(0)
	checks.Add("postgres", postgres.Check(pool))
	checks.Add("redis", platformredis.Check(rdb))

	var publisher application.Publisher = kafkaadapter.NopPublisher{}
	var (
		producer *platformkafka.Producer
		relay    *outbox.Relay
	)
	if cfg.KafkaEnabled {
		if producer, err = platformkafka.NewProducer(platformkafka.ProducerConfig{Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName}, reg); err != nil {
			return err
		}
		closers.Add("kafka producer", producer.Close)
		checks.Add("kafka", producer.Ping)
		// profile.events go to profile.outbox in the transaction of each
		// change; the relay moves them to Kafka.
		sink, err := outbox.NewSink(pool, pgadapter.OutboxTable)
		if err != nil {
			return err
		}
		publisher = kafkaadapter.NewPublisher(sink, config.ServiceName)
		if relay, err = outbox.NewRelay(pool, producer, outbox.RelayConfig{Table: pgadapter.OutboxTable}, log, reg); err != nil {
			return err
		}
	}

	// 5. Application.
	app := application.New(pgadapter.New(pool), publisher, postgres.Transactor{Pool: pool}, log)

	// 6. Consumers: auth.events → profile creation. Runs until shutdown;
	// offsets are committed only after the profile is stored.
	consumerDone := make(chan error, 1)
	if cfg.KafkaEnabled {
		consumer, err := platformkafka.NewConsumer(platformkafka.ConsumerConfig{
			Brokers: cfg.KafkaBrokers, ClientID: config.ServiceName, Group: config.ConsumerGroup,
			Topics: []string{events.TopicAuthEvents}, MaxRetries: 5, RetryBackoff: 500 * time.Millisecond, DLQ: true,
		}, kafkaadapter.AuthEventsHandler(app), log, platformkafka.NewConsumerMetrics(reg))
		if err != nil {
			return err
		}
		go func() { consumerDone <- consumer.Run(ctx) }()
	} else {
		close(consumerDone)
	}

	// 7. Handlers. Access tokens are verified against Auth's JWKS.
	keys := authn.NewRemoteKeys(cfg.JWKSURL, httpclient.New(httpclient.Config{Timeout: 3 * time.Second}))
	verifier := authn.NewVerifier(keys, authn.NewRedisRevocations(rdb))
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, verifier, log).Register(mux)
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
	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		log.Error("consumer stopped with error", logger.Err(err))
	}
	log.Info("user profile service stopped")
	return nil
}
