//go:build integration

package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// REDIS_ADDR=localhost:6379 go test -tags integration ./internal/adapters/cache
func caches(t *testing.T) Caches {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR is not set")
	}
	cl, err := redis.Open(context.Background(), redis.Config{Addr: addr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return New(cl, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)), redis.NewCacheMetrics(prometheus.NewRegistry()))
}

// countingTracks is an in-memory TrackRepository that counts reads.
type countingTracks struct {
	ports.TrackRepository
	tracks map[uuid.UUID]domain.Track
	gets   int
	lists  int
}

func (r *countingTracks) Get(_ context.Context, id uuid.UUID) (domain.Track, error) {
	r.gets++
	t, ok := r.tracks[id]
	if !ok {
		return domain.Track{}, domain.ErrTrackNotFound
	}
	return t, nil
}

func (r *countingTracks) ListByAlbum(_ context.Context, albumID uuid.UUID) ([]domain.Track, error) {
	r.lists++
	var out []domain.Track
	for _, t := range r.tracks {
		if t.AlbumID == albumID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *countingTracks) Create(_ context.Context, t domain.Track) error {
	r.tracks[t.ID] = t
	return nil
}

func (r *countingTracks) Update(_ context.Context, t domain.Track) error {
	r.tracks[t.ID] = t
	return nil
}

func TestTracksReadThroughAndInvalidation(t *testing.T) {
	ctx := context.Background()
	albumID, trackID := uuid.New(), uuid.New()
	src := &countingTracks{tracks: map[uuid.UUID]domain.Track{
		trackID: {ID: trackID, AlbumID: albumID, Title: "Glass Tide", Status: domain.TrackStatusReady},
	}}
	repo := NewTracks(src, caches(t))

	for range 3 {
		got, err := repo.Get(ctx, trackID)
		if err != nil || got.Title != "Glass Tide" {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		if list, err := repo.ListByAlbum(ctx, albumID); err != nil || len(list) != 1 {
			t.Fatalf("ListByAlbum = %v, %v", list, err)
		}
	}
	if src.gets != 1 || src.lists != 1 {
		t.Fatalf("source reads: gets=%d lists=%d, want 1 and 1", src.gets, src.lists)
	}

	updated := src.tracks[trackID]
	updated.Title = "Glass Tide (Remastered)"
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, trackID); got.Title != updated.Title {
		t.Fatalf("after update Get title = %q", got.Title)
	}

	second := domain.Track{ID: uuid.New(), AlbumID: albumID, Title: "Low Orbit"}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	if list, _ := repo.ListByAlbum(ctx, albumID); len(list) != 2 {
		t.Fatalf("after create album has %d tracks, want 2", len(list))
	}
}

func TestTracksNotFoundIsNotCached(t *testing.T) {
	src := &countingTracks{tracks: map[uuid.UUID]domain.Track{}}
	repo := NewTracks(src, caches(t))
	id := uuid.New()
	for range 2 {
		if _, err := repo.Get(context.Background(), id); !errors.Is(err, domain.ErrTrackNotFound) {
			t.Fatalf("err = %v, want ErrTrackNotFound", err)
		}
	}
	if src.gets != 2 {
		t.Fatalf("source gets = %d, want 2 (misses are not cached)", src.gets)
	}
}
