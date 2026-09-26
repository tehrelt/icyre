package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/workers/analytics/internal/domain"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeCatalog struct {
	tracks map[uuid.UUID]domain.TrackInfo
	calls  [][]uuid.UUID
	err    error
}

func (f *fakeCatalog) Tracks(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.TrackInfo, error) {
	f.calls = append(f.calls, ids)
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]domain.TrackInfo{}
	for _, id := range ids {
		if t, ok := f.tracks[id]; ok {
			out[id] = t
		}
	}
	return out, nil
}

type fakeStore struct {
	inserted [][]domain.Event
	tokens   []string
	days     []time.Time
	failDay  time.Time
}

func (f *fakeStore) InsertEvents(_ context.Context, evs []domain.Event, token string) error {
	f.inserted = append(f.inserted, evs)
	f.tokens = append(f.tokens, token)
	return nil
}

func (f *fakeStore) RecomputeDay(_ context.Context, day time.Time) error {
	f.days = append(f.days, day)
	if day.Equal(f.failDay) {
		return errors.New("boom")
	}
	return nil
}

func TestIngestEnrichesAndDeduplicatesLookups(t *testing.T) {
	known, unknown := uuid.New(), uuid.New()
	album, artist := uuid.New(), uuid.New()
	cat := &fakeCatalog{tracks: map[uuid.UUID]domain.TrackInfo{known: {AlbumID: album, ArtistIDs: []uuid.UUID{artist}}}}
	store := &fakeStore{}
	evs := []domain.Event{
		{EventID: uuid.New(), TrackID: known}, {EventID: uuid.New(), TrackID: known}, {EventID: uuid.New(), TrackID: unknown},
	}
	ing := NewIngester(cat, store, discard)
	if err := ing.Ingest(context.Background(), evs); err != nil {
		t.Fatal(err)
	}
	if len(cat.calls) != 1 || len(cat.calls[0]) != 2 {
		t.Fatalf("catalog calls %v", cat.calls)
	}
	got := store.inserted[0]
	if got[0].AlbumID != album || !slices.Equal(got[1].ArtistIDs, []uuid.UUID{artist}) || got[2].AlbumID != uuid.Nil {
		t.Fatalf("enriched %+v", got)
	}
	if evs[0].AlbumID != uuid.Nil {
		t.Fatal("input mutated")
	}

	// A retry of the same batch carries the same token; another batch does not.
	_ = ing.Ingest(context.Background(), evs)
	_ = ing.Ingest(context.Background(), evs[:2])
	if store.tokens[0] != store.tokens[1] || store.tokens[0] == store.tokens[2] {
		t.Fatalf("tokens %v", store.tokens)
	}
}

func TestIngestFailsWhenCatalogFails(t *testing.T) {
	store := &fakeStore{}
	err := NewIngester(&fakeCatalog{err: errors.New("down")}, store, discard).
		Ingest(context.Background(), []domain.Event{{EventID: uuid.New(), TrackID: uuid.New()}})
	if err == nil || len(store.inserted) != 0 {
		t.Fatalf("err %v inserted %d", err, len(store.inserted))
	}
}

func TestTrackCache(t *testing.T) {
	known, unknown := uuid.New(), uuid.New()
	cat := &fakeCatalog{tracks: map[uuid.UUID]domain.TrackInfo{known: {AlbumID: uuid.New()}}}
	c := NewTrackCache(cat, time.Minute, 10)
	now := time.Now()
	c.now = func() time.Time { return now }
	ctx := context.Background()

	for range 2 {
		got, err := c.Tracks(ctx, []uuid.UUID{known, unknown})
		if err != nil || len(got) != 1 {
			t.Fatalf("got %v err %v", got, err)
		}
	}
	if len(cat.calls) != 1 {
		t.Fatalf("unknown tracks must be cached too: %d calls", len(cat.calls))
	}
	now = now.Add(2 * time.Minute)
	if _, err := c.Tracks(ctx, []uuid.UUID{known}); err != nil || len(cat.calls) != 2 {
		t.Fatalf("expired entry not refreshed: %d calls, %v", len(cat.calls), err)
	}
}

func TestRecomputeWalksDaysAndReportsFailures(t *testing.T) {
	store := &fakeStore{failDay: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)}
	a := NewAggregator(store, 2, discard, nil)
	a.now = func() time.Time { return time.Date(2026, 9, 27, 23, 30, 0, 0, time.FixedZone("X", -2*3600)) } // 28th in UTC

	err := a.RecomputeRecent(context.Background())
	if err == nil {
		t.Fatal("want the failed day reported")
	}
	var days []string
	for _, d := range store.days {
		days = append(days, d.Format(time.DateOnly))
	}
	if !slices.Equal(days, []string{"2026-09-26", "2026-09-27", "2026-09-28"}) {
		t.Fatalf("days %v", days)
	}
	if err := a.Recompute(context.Background(), time.Now(), time.Now().AddDate(0, 0, -1)); err == nil {
		t.Fatal("want an error for a reversed range")
	}
}
