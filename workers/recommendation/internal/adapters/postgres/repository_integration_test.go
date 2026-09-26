//go:build integration

package postgres

import (
	"context"
	"io"
	"log/slog"
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
)

// Requires RECOMMENDATION_TEST_DATABASE_DSN (make up-core).
func newRepo(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("RECOMMENDATION_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("RECOMMENDATION_TEST_DATABASE_DSN not set")
	}
	ctx := context.Background()
	pool, err := platformpg.Open(ctx, platformpg.Config{DSN: dsn}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return New(pool)
}

func TestLikesIgnoreStaleEvents(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	user, track, album := uuid.New(), uuid.New(), uuid.New()
	t0 := time.Now().UTC().Truncate(time.Millisecond)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(r.SaveLike(ctx, KindTrack, user, track, t0.Add(time.Minute)))
	must(r.RemoveLike(ctx, KindTrack, user, track, t0)) // older than the save: ignored
	must(r.SaveLike(ctx, KindAlbum, user, album, t0))
	must(r.SaveLike(ctx, KindAlbum, user, album, t0)) // redelivery
	tracks, albums, err := r.Likes(ctx)
	must(err)
	if !tracks[user][track] || !albums[user][album] {
		t.Fatalf("likes %v %v", tracks[user], albums[user])
	}

	must(r.RemoveLike(ctx, KindTrack, user, track, t0.Add(2*time.Minute)))
	tracks, _, err = r.Likes(ctx)
	must(err)
	if tracks[user][track] {
		t.Fatal("removal not applied")
	}
	// A stale save delivered after the removal (redrive) must not resurrect it.
	must(r.SaveLike(ctx, KindTrack, user, track, t0.Add(time.Minute)))
	tracks, _, err = r.Likes(ctx)
	must(err)
	if tracks[user][track] {
		t.Fatal("stale save resurrected a removed like")
	}
	if err := r.SaveLike(ctx, "playlist", user, track, t0); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestFeaturesKeepLatestMaster(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	track := uuid.New()
	t0 := time.Now().UTC()
	bpm := 128.0
	if err := r.SaveFeatures(ctx, Features{TrackID: track, BPM: &bpm, IntegratedLUFS: -9, LoudnessRangeLU: 6, AnalyzerVersion: "v1", UploadedAt: t0}); err != nil {
		t.Fatal(err)
	}
	// An older master arriving late does not overwrite the newer one.
	if err := r.SaveFeatures(ctx, Features{TrackID: track, IntegratedLUFS: -30, AnalyzerVersion: "v1", UploadedAt: t0.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	sounds, err := r.Sounds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s := sounds[track]
	// 128 BPM → (128-60)/140; -9 LUFS → 61/70: the newer master's values.
	if s == nil || s.Version != "v1" || math.Abs(s.Vector[0]-68.0/140) > 1e-9 || math.Abs(s.Vector[1]-61.0/70) > 1e-9 {
		t.Fatalf("sound %+v", s)
	}
}
