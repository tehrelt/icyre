// Command analytics owns the ClickHouse analytics store.
//
//	analytics migrate  apply the schema (libs/contracts/analytics) to ClickHouse
//
// Consuming playback.events and the aggregation jobs arrive with EPIC-028.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/libs/platform/logger"
	"github.com/tehrelt/icyre/libs/platform/shutdown"
	"github.com/tehrelt/icyre/workers/analytics/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "analytics:", err)
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

	if len(args) != 1 || args[0] != "migrate" {
		return fmt.Errorf("usage: analytics migrate")
	}
	return migrate(ctx, clickhouse.New(cfg.ClickHouse, nil), log)
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
