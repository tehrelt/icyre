//go:build integration

package postgres

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PLAYLIST_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("PLAYLIST_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("playlist_it_%d", time.Now().UnixNano())
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
	owner := uuid.New()
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	p := domain.Playlist{ID: uuid.New(), OwnerID: owner, Title: "Late night", CreatedAt: now, UpdatedAt: now}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Concurrent appends get distinct, dense positions.
	ids := make([]uuid.UUID, 8)
	var wg sync.WaitGroup
	for i := range ids {
		ids[i] = uuid.New()
		wg.Go(func() {
			if err := repo.AppendTrack(ctx, p.ID, domain.Track{TrackID: ids[i], AddedBy: owner, AddedAt: now.Add(time.Minute)}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	_ = repo.AppendTrack(ctx, p.ID, domain.Track{TrackID: ids[0], AddedBy: owner, AddedAt: now}) // duplicate: no-op
	tracks, err := repo.Tracks(ctx, p.ID)
	if err != nil || len(tracks) != 8 || tracks[0].Position != 1 || tracks[7].Position != 8 {
		t.Fatalf("tracks %+v %v", tracks, err)
	}

	if err := repo.RemoveTrack(ctx, p.ID, tracks[3].TrackID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, p.ID)
	if err != nil || got.TrackCount != 7 || !got.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("get %+v %v", got, err)
	}
	_ = repo.AppendTrack(ctx, p.ID, domain.Track{TrackID: uuid.New(), AddedBy: owner, AddedAt: now})
	if tracks, _ := repo.Tracks(ctx, p.ID); tracks[len(tracks)-1].Position != 9 {
		t.Fatalf("append after a gap: %+v", tracks[len(tracks)-1])
	}

	mine, err := repo.ByOwner(ctx, owner)
	if err != nil || len(mine) != 1 || mine[0].TrackCount != 8 {
		t.Fatalf("mine %+v %v", mine, err)
	}
	if _, err := repo.Get(ctx, uuid.New()); err != domain.ErrNotFound {
		t.Fatal(err)
	}
}
