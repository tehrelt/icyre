//go:build integration

package clickhouse

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/libs/platform/clickhouse"
	"github.com/tehrelt/icyre/workers/analytics/internal/domain"
)

// newStore migrates a throwaway database on CLICKHOUSE_URL (make up-core).
func newStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("CLICKHOUSE_URL")
	if url == "" {
		t.Skip("CLICKHOUSE_URL not set")
	}
	ctx := context.Background()
	cfg := clickhouse.Config{URL: url, Username: os.Getenv("CLICKHOUSE_USERNAME"), Password: os.Getenv("CLICKHOUSE_PASSWORD")}
	db := fmt.Sprintf("it_analytics_%d", time.Now().UnixNano())
	if err := clickhouse.New(cfg, nil).Exec(ctx, "CREATE DATABASE "+db, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clickhouse.New(cfg, nil).Exec(context.Background(), "DROP DATABASE IF EXISTS "+db, nil) })
	cfg.Database = db
	ch := clickhouse.New(cfg, nil)
	ms, err := clickhouse.LoadMigrations(analytics.Migrations, analytics.MigrationsDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ch.Migrate(ctx, ms); err != nil {
		t.Fatal(err)
	}
	return New(ch)
}

func TestIngestRecomputeReport(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	day1 := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	hit, deep := uuid.New(), uuid.New()              // tracks
	solo, feat := uuid.New(), uuid.New()             // artists: hit is by solo+feat, deep by solo
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New() // listeners

	var evs []domain.Event
	play := func(user, track uuid.UUID, artists []uuid.UUID, end string, at time.Time) {
		pb := uuid.New()
		base := domain.Event{PlaybackID: pb, UserID: user, TrackID: track, ArtistIDs: artists, Source: "album:x", DurationMs: 200_000, At: at}
		started, ended := base, base
		started.EventID, started.Type = uuid.New(), "playback.started"
		ended.EventID, ended.Type, ended.ListenedMs = uuid.New(), end, 100_000
		evs = append(evs, started, ended)
	}
	play(u1, hit, []uuid.UUID{solo, feat}, "playback.finished", day1)
	play(u2, hit, []uuid.UUID{solo, feat}, "playback.finished", day1)
	play(u1, hit, []uuid.UUID{solo, feat}, "playback.skipped", day2)
	play(u3, deep, []uuid.UUID{solo}, "playback.finished", day2)

	if err := s.InsertEvents(ctx, evs, "b1"); err != nil {
		t.Fatal(err)
	}
	// Redelivery in another batch must not double count.
	if err := s.InsertEvents(ctx, evs[:4], "b2"); err != nil {
		t.Fatal(err)
	}
	for range 2 { // recomputing is idempotent
		for _, d := range []time.Time{day1, day2} {
			if err := s.RecomputeDay(ctx, d); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A day without events leaves no totals row.
	if err := s.RecomputeDay(ctx, day2.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}

	one, err := s.Report(ctx, day1, day1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if one.Totals != (domain.Stats{Plays: 2, Completions: 2, UniqueListeners: 2, ListenedMs: 200_000}) || one.DailyActiveListeners != 2 {
		t.Fatalf("day1 %+v", one)
	}

	rep, err := s.Report(ctx, day1, day2.AddDate(0, 0, 1), 10)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Stats{Plays: 4, Completions: 3, Skips: 1, UniqueListeners: 3, ListenedMs: 400_000}
	if rep.Totals != want || rep.CompletionRate != 0.75 || rep.DailyActiveListeners != 2 {
		t.Fatalf("totals %+v rate %v dal %v", rep.Totals, rep.CompletionRate, rep.DailyActiveListeners)
	}
	if len(rep.TopTracks) != 2 || rep.TopTracks[0].ID != hit || rep.TopTracks[0].Plays != 3 ||
		rep.TopTracks[0].UniqueListeners != 2 || rep.TopTracks[0].Skips != 1 {
		t.Fatalf("top tracks %+v", rep.TopTracks)
	}
	if len(rep.TopArtists) != 2 || rep.TopArtists[0].ID != solo || rep.TopArtists[0].Plays != 4 ||
		rep.TopArtists[0].UniqueListeners != 3 || rep.TopArtists[1].ID != feat || rep.TopArtists[1].Plays != 3 {
		t.Fatalf("top artists %+v", rep.TopArtists)
	}

	top1, err := s.Report(ctx, day1, day2, 1)
	if err != nil || len(top1.TopTracks) != 1 || len(top1.TopArtists) != 1 {
		t.Fatalf("limit: %+v %v", top1, err)
	}
}
