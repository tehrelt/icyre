// Command auth runs the Auth Service.
//
//	auth          serve the HTTP API
//	auth migrate  apply database migrations and exit
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
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/outbox"
	"github.com/tehrelt/icyre/libs/platform/postgres"
	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/libs/platform/telemetry"
	"github.com/tehrelt/icyre/services/auth/internal/adapters/crypto"
	httpadapter "github.com/tehrelt/icyre/services/auth/internal/adapters/http"
	kafkaadapter "github.com/tehrelt/icyre/services/auth/internal/adapters/kafka"
	pgadapter "github.com/tehrelt/icyre/services/auth/internal/adapters/postgres"
	redisadapter "github.com/tehrelt/icyre/services/auth/internal/adapters/redis"
	"github.com/tehrelt/icyre/services/auth/internal/application"
	"github.com/tehrelt/icyre/services/auth/internal/config"
	"github.com/tehrelt/icyre/services/auth/internal/ports"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "auth:", err)
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

	// Events go to auth.outbox in the transaction of each change; the relay
	// moves them to Kafka (at-least-once end to end).
	var (
		publisher ports.EventPublisher = kafkaadapter.NopPublisher{}
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
		publisher = kafkaadapter.NewPublisher(sink, config.ServiceName)
		if relay, err = outbox.NewRelay(pool, producer, outbox.RelayConfig{Table: pgadapter.OutboxTable}, log, reg); err != nil {
			return err
		}
	}

	// 5. Keys and adapters.
	var key crypto.SigningKey
	if cfg.SigningKey != "" {
		if key, err = crypto.ParseSigningKey(cfg.SigningKeyID, cfg.SigningKey); err != nil {
			return err
		}
	} else {
		// A fresh kid per process: verifiers caching the JWKS see an unknown
		// kid after a restart and refetch instead of rejecting signatures.
		kid := cfg.SigningKeyID + "-" + time.Now().UTC().Format("20060102T150405")
		if key, err = crypto.GenerateSigningKey(kid); err != nil {
			return err
		}
		log.Warn("AUTH_SIGNING_KEY not set: using an ephemeral key, tokens die with the process (local only)", "kid", kid)
	}
	issuer := crypto.NewJWTIssuer(key, cfg.AccessTTL)
	revocations := authn.NewRedisRevocations(rdb)
	verifier := authn.NewVerifier(authn.StaticKeys{key.ID: key.Public()}, revocations)

	// 6. Application.
	app, err := application.New(application.Deps{
		Accounts:   pgadapter.NewAccounts(pool),
		Sessions:   pgadapter.NewSessions(pool),
		Hasher:     crypto.NewArgon2Hasher(crypto.DefaultArgon2),
		Tokens:     issuer,
		Refresh:    crypto.RefreshTokens{},
		Revocation: revocations,
		Throttle:   redisadapter.NewLoginThrottle(rdb, cfg.LoginMaxFailures, cfg.LoginWindow),
		Publisher:  publisher,
		Tx:         postgres.Transactor{Pool: pool},
		Log:        log,
		SessionTTL: cfg.SessionTTL,
	})
	if err != nil {
		return err
	}

	// 7. Handlers.
	mux := http.NewServeMux()
	checks.Register(mux)
	mux.Handle("GET /metrics", telemetry.MetricsHandler(reg))
	httpadapter.NewHandler(app, verifier, issuer.JWKS(), httpadapter.Options{SecureCookies: cfg.SecureCookies}, log).Register(mux)
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
	log.Info("auth service stopped")
	return nil
}
