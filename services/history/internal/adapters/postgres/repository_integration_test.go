//go:build integration

package postgres

import (
	"context"
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
	"github.com/tehrelt/icyre/services/history/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("HISTORY_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("HISTORY_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("history_it_%d", time.Now().UnixNano())
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
	user := uuid.New()
	t0 := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	listen := func(source string, at time.Duration) domain.Listen {
		return domain.Listen{PlaybackID: uuid.New(), UserID: user, TrackID: uuid.New(), Source: source, DurationMs: 200000, ListenedMs: 60000, PlayedAt: t0.Add(at)}
	}
	ls := []domain.Listen{listen("album:a", 0), listen("album:b", time.Minute), listen("album:a", 2*time.Minute), listen("", 3*time.Minute)}
	for _, l := range ls {
		if ok, err := repo.Record(ctx, l); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	if ok, _ := repo.Record(ctx, ls[0]); ok {
		t.Fatal("duplicate playback recorded")
	}
	page, err := repo.Tracks(ctx, user, nil, 2)
	if err != nil || len(page) != 2 || page[0].PlaybackID != ls[3].PlaybackID {
		t.Fatal(page, err)
	}
	last := page[1]
	rest, _ := repo.Tracks(ctx, user, &domain.Cursor{PlayedAt: last.PlayedAt, PlaybackID: last.PlaybackID}, 10)
	if len(rest) != 2 || rest[1].PlaybackID != ls[0].PlaybackID {
		t.Fatalf("page 2: %+v", rest)
	}
	src, err := repo.RecentSources(ctx, user, 10)
	if err != nil || len(src) != 2 || src[0].Source != "album:a" || !src[0].PlayedAt.Equal(t0.Add(2*time.Minute)) || src[1].Source != "album:b" {
		t.Fatalf("sources %+v %v", src, err)
	}
	if other, _ := repo.Tracks(ctx, uuid.New(), nil, 10); len(other) != 0 {
		t.Fatal("history leaked across users")
	}
}
