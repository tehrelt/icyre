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
	"github.com/tehrelt/icyre/services/library/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("LIBRARY_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("LIBRARY_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("library_it_%d", time.Now().UnixNano())
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
	user, other := uuid.New(), uuid.New()
	t0 := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	var ids []uuid.UUID
	for i := range 5 {
		id := uuid.New()
		ids = append(ids, id)
		// Two items share a timestamp: the keyset must still page correctly.
		at := t0.Add(time.Duration(min(i, 3)) * time.Minute)
		if ch, err := repo.Save(ctx, domain.Item{UserID: user, Kind: domain.KindTrack, EntityID: id, SavedAt: at}); err != nil || !ch.Changed {
			t.Fatal(ch, err)
		}
	}
	ch, err := repo.Save(ctx, domain.Item{UserID: user, Kind: domain.KindTrack, EntityID: ids[0], SavedAt: t0.Add(time.Hour)})
	if err != nil || ch.Changed || !ch.Item.SavedAt.Equal(t0) {
		t.Fatalf("re-save must keep the original time: %+v %v", ch, err)
	}
	_, _ = repo.Save(ctx, domain.Item{UserID: user, Kind: domain.KindAlbum, EntityID: uuid.New(), SavedAt: t0})
	_, _ = repo.Save(ctx, domain.Item{UserID: other, Kind: domain.KindTrack, EntityID: ids[0], SavedAt: t0})

	var seen []uuid.UUID
	var after *domain.Cursor
	for {
		page, err := repo.List(ctx, user, domain.KindTrack, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range page {
			seen = append(seen, it.EntityID)
		}
		if len(page) < 2 {
			break
		}
		last := page[len(page)-1]
		after = &domain.Cursor{SavedAt: last.SavedAt, EntityID: last.EntityID}
	}
	if len(seen) != 5 || seen[4] != ids[0] {
		t.Fatalf("pagination: %v", seen)
	}
	uniq := map[uuid.UUID]bool{}
	for _, id := range seen {
		uniq[id] = true
	}
	if len(uniq) != 5 {
		t.Fatalf("duplicates across pages: %v", seen)
	}

	got, err := repo.Contains(ctx, user, domain.KindTrack, []uuid.UUID{ids[1], uuid.New()})
	if err != nil || len(got) != 1 || got[0] != ids[1] {
		t.Fatal(got, err)
	}
	c, _ := repo.Counts(ctx, user)
	if c.Tracks != 5 || c.Albums != 1 {
		t.Fatal(c)
	}

	if ch, err := repo.Remove(ctx, user, domain.KindTrack, ids[1], t0); err != nil || !ch.Changed {
		t.Fatal(ch, err)
	}
	if ch, _ := repo.Remove(ctx, user, domain.KindTrack, ids[1], t0); ch.Changed {
		t.Fatal("second remove changed something")
	}
	if c, _ := repo.Counts(ctx, other); c.Tracks != 1 {
		t.Fatalf("other user's library touched: %+v", c)
	}
}
