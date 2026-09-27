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
	"github.com/tehrelt/icyre/services/social/internal/domain"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SOCIAL_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("SOCIAL_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("social_it_%d", time.Now().UnixNano())
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

func counts(t *testing.T, repo *Repository, typ domain.TargetType, id uuid.UUID) domain.Counts {
	t.Helper()
	c, err := repo.Counts(context.Background(), domain.Target{Type: typ, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRepository(t *testing.T) {
	repo := New(newTestDB(t))
	ctx := context.Background()
	me, friend, artist := uuid.New(), uuid.New(), uuid.New()
	t0 := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	user := domain.Target{Type: domain.TargetUser, ID: friend}
	art := domain.Target{Type: domain.TargetArtist, ID: artist}

	ch, err := repo.Follow(ctx, domain.Follow{FollowerID: me, Target: user, FollowedAt: t0})
	if err != nil || !ch.Changed {
		t.Fatalf("follow: %+v %v", ch, err)
	}
	ch, err = repo.Follow(ctx, domain.Follow{FollowerID: me, Target: user, FollowedAt: t0.Add(time.Hour)})
	if err != nil || ch.Changed || !ch.Follow.FollowedAt.Equal(t0) {
		t.Fatalf("repeat follow keeps original time: %+v %v", ch, err)
	}
	if _, err := repo.Follow(ctx, domain.Follow{FollowerID: me, Target: art, FollowedAt: t0.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Follow(ctx, domain.Follow{FollowerID: friend, Target: art, FollowedAt: t0.Add(2 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Follow(ctx, domain.Follow{FollowerID: me, Target: domain.Target{Type: domain.TargetUser, ID: me}, FollowedAt: t0}); err == nil {
		t.Fatal("self follow must violate the check constraint")
	}

	if c := counts(t, repo, domain.TargetUser, me); c != (domain.Counts{FollowingUsers: 1, FollowingArtists: 1}) {
		t.Fatalf("my counts: %+v", c)
	}
	if c := counts(t, repo, domain.TargetUser, friend); c != (domain.Counts{Followers: 1, FollowingArtists: 1}) {
		t.Fatalf("friend counts: %+v", c)
	}
	if c := counts(t, repo, domain.TargetArtist, artist); c.Followers != 2 {
		t.Fatalf("artist counts: %+v", c)
	}
	if c := counts(t, repo, domain.TargetArtist, uuid.New()); c != (domain.Counts{}) {
		t.Fatalf("unknown counts: %+v", c)
	}

	// Followers: newest first, keyset pages.
	fs, err := repo.Followers(ctx, art, nil, 1)
	if err != nil || len(fs) != 1 || fs[0].FollowerID != friend {
		t.Fatalf("followers page 1: %+v %v", fs, err)
	}
	fs, err = repo.Followers(ctx, art, &domain.Cursor{At: fs[0].FollowedAt, ID: fs[0].FollowerID}, 10)
	if err != nil || len(fs) != 1 || fs[0].FollowerID != me {
		t.Fatalf("followers page 2: %+v %v", fs, err)
	}
	fs, err = repo.Following(ctx, me, domain.TargetArtist, nil, 10)
	if err != nil || len(fs) != 1 || fs[0].Target != art {
		t.Fatalf("following: %+v %v", fs, err)
	}
	ids, err := repo.Contains(ctx, me, domain.TargetArtist, []uuid.UUID{artist, friend})
	if err != nil || len(ids) != 1 || ids[0] != artist {
		t.Fatalf("contains: %v %v", ids, err)
	}

	for i, want := range []bool{true, false} {
		ch, err := repo.Unfollow(ctx, me, art, t0.Add(time.Hour))
		if err != nil || ch.Changed != want {
			t.Fatalf("unfollow #%d: %+v %v", i, ch, err)
		}
	}
	if c := counts(t, repo, domain.TargetUser, me); c != (domain.Counts{FollowingUsers: 1}) {
		t.Fatalf("my counts after unfollow: %+v", c)
	}
	if c := counts(t, repo, domain.TargetArtist, artist); c.Followers != 1 {
		t.Fatalf("artist counts after unfollow: %+v", c)
	}
}

// Concurrent follows of one artist must not lose counter updates.
func TestCountersUnderConcurrency(t *testing.T) {
	repo := New(newTestDB(t))
	ctx := context.Background()
	art := domain.Target{Type: domain.TargetArtist, ID: uuid.New()}
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Go(func() {
			_, err := repo.Follow(ctx, domain.Follow{FollowerID: uuid.New(), Target: art, FollowedAt: time.Now().UTC()})
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if c := counts(t, repo, domain.TargetArtist, art.ID); c.Followers != n {
		t.Fatalf("followers: %d want %d", c.Followers, n)
	}
}
