package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

var schemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Migrate applies every pending migration found in fsys to the given schema.
//
// Each domain owns one schema (catalog, playlist, …) and its own version
// table <schema>.schema_migrations, so services migrate independently even
// while they share one PostgreSQL cluster.
func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string, fsys fs.FS, log *slog.Logger) error {
	if !schemaName.MatchString(schema) {
		return fmt.Errorf("invalid schema name %q", schema)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		return fmt.Errorf("create schema %s: %w", schema, err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	store, err := database.NewStore(database.DialectPostgres, schema+".schema_migrations")
	if err != nil {
		return fmt.Errorf("migration store: %w", err)
	}
	provider, err := goose.NewProvider("", db, fsys, goose.WithStore(store))
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	for _, r := range results {
		log.InfoContext(ctx, "migration applied",
			"schema", schema, "migration", r.Source.Version, "path", r.Source.Path, "duration", r.Duration)
	}
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if len(results) == 0 {
		log.InfoContext(ctx, "schema is up to date", "schema", schema)
	}
	return nil
}
