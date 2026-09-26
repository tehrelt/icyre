//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MEDIA_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("MEDIA_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("media_it_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestRepository(t *testing.T) {
	repo := New(newTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	u := domain.Upload{
		ID: uuid.New(), TrackID: uuid.New(), UploaderID: uuid.New(), ContentType: "audio/flac", SizeBytes: 42,
		SHA256: make([]byte, 32), Key: "tracks/t/original/u.flac", Status: domain.StatusPending, CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, u.ID)
	if err != nil || got.Key != u.Key || got.Status != domain.StatusPending || got.CompletedAt != nil || len(got.SHA256) != 32 {
		t.Fatalf("%+v %v", got, err)
	}
	failed := u.Settle("SIZE_MISMATCH", now)
	if ok, err := repo.Settle(ctx, failed); !ok || err != nil {
		t.Fatal(ok, err)
	}
	// Settled once: a second (concurrent) settle changes nothing.
	if ok, err := repo.Settle(ctx, u.Settle("", now)); ok || err != nil {
		t.Fatal(ok, err)
	}
	got, _ = repo.Get(ctx, u.ID)
	if got.Status != domain.StatusFailed || got.FailureReason != "SIZE_MISMATCH" || got.CompletedAt == nil {
		t.Fatalf("%+v", got)
	}
	if _, err := repo.Get(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
}
