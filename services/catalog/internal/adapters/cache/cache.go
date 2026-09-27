// Package cache puts a Redis read-through cache in front of the Catalog
// repositories (CACHE_ENABLED, EPIC-038 experiment "with / without Redis").
//
// Reads by ID, album track lists and the genre list are cached with a TTL;
// writes pass through and invalidate the affected keys. The invalidation
// runs inside the write transaction, so a reader racing the commit can cache
// the previous version: staleness is bounded by the TTL (keep it short).
// GetForUpdate and batch lookups always hit PostgreSQL.
package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// Caches are the key namespaces of the Catalog cache.
type Caches struct {
	Artists     *redis.Cache[domain.Artist]
	Albums      *redis.Cache[domain.Album]
	Tracks      *redis.Cache[domain.Track]
	AlbumTracks *redis.Cache[[]domain.Track]
	Genres      *redis.Cache[[]domain.Genre]
	Log         *slog.Logger
}

// New creates the caches over cl, every entry living ttl.
func New(cl *redis.Client, ttl time.Duration, log *slog.Logger, m *redis.CacheMetrics) Caches {
	return Caches{
		Artists:     redis.NewCache[domain.Artist](cl, "catalog-artist", ttl, log, m),
		Albums:      redis.NewCache[domain.Album](cl, "catalog-album", ttl, log, m),
		Tracks:      redis.NewCache[domain.Track](cl, "catalog-track", ttl, log, m),
		AlbumTracks: redis.NewCache[[]domain.Track](cl, "catalog-album-tracks", ttl, log, m),
		Genres:      redis.NewCache[[]domain.Genre](cl, "catalog-genres", ttl, log, m),
		Log:         log,
	}
}

// genresKey is the single entry of the genre list.
const genresKey = "all"

// invalidate drops keys; a Redis failure is logged, the TTL expires the entry.
func invalidate[T any](ctx context.Context, log *slog.Logger, c *redis.Cache[T], ids ...uuid.UUID) {
	for _, id := range ids {
		if err := c.Delete(ctx, id.String()); err != nil && log != nil {
			log.WarnContext(ctx, "cache invalidation failed", "id", id, "error", err)
		}
	}
}

// Artists caches ArtistRepository.Get.
type Artists struct {
	ports.ArtistRepository
	c Caches
}

// NewArtists wraps next.
func NewArtists(next ports.ArtistRepository, c Caches) *Artists {
	return &Artists{ArtistRepository: next, c: c}
}

// Get implements ports.ArtistRepository.
func (r *Artists) Get(ctx context.Context, id uuid.UUID) (domain.Artist, error) {
	return r.c.Artists.GetOrLoad(ctx, id.String(), func(ctx context.Context) (domain.Artist, error) {
		return r.ArtistRepository.Get(ctx, id)
	})
}

// Albums caches AlbumRepository.Get. Albums are immutable after creation.
type Albums struct {
	ports.AlbumRepository
	c Caches
}

// NewAlbums wraps next.
func NewAlbums(next ports.AlbumRepository, c Caches) *Albums {
	return &Albums{AlbumRepository: next, c: c}
}

// Get implements ports.AlbumRepository.
func (r *Albums) Get(ctx context.Context, id uuid.UUID) (domain.Album, error) {
	return r.c.Albums.GetOrLoad(ctx, id.String(), func(ctx context.Context) (domain.Album, error) {
		return r.AlbumRepository.Get(ctx, id)
	})
}

// Tracks caches TrackRepository.Get and ListByAlbum and invalidates them on
// writes.
type Tracks struct {
	ports.TrackRepository
	c Caches
}

// NewTracks wraps next.
func NewTracks(next ports.TrackRepository, c Caches) *Tracks {
	return &Tracks{TrackRepository: next, c: c}
}

// Get implements ports.TrackRepository.
func (r *Tracks) Get(ctx context.Context, id uuid.UUID) (domain.Track, error) {
	return r.c.Tracks.GetOrLoad(ctx, id.String(), func(ctx context.Context) (domain.Track, error) {
		return r.TrackRepository.Get(ctx, id)
	})
}

// ListByAlbum implements ports.TrackRepository.
func (r *Tracks) ListByAlbum(ctx context.Context, albumID uuid.UUID) ([]domain.Track, error) {
	return r.c.AlbumTracks.GetOrLoad(ctx, albumID.String(), func(ctx context.Context) ([]domain.Track, error) {
		return r.TrackRepository.ListByAlbum(ctx, albumID)
	})
}

// Create implements ports.TrackRepository: the album's track list changes.
func (r *Tracks) Create(ctx context.Context, t domain.Track) error {
	if err := r.TrackRepository.Create(ctx, t); err != nil {
		return err
	}
	invalidate(ctx, r.c.Log, r.c.AlbumTracks, t.AlbumID)
	return nil
}

// Update implements ports.TrackRepository: the track and its album's list change.
func (r *Tracks) Update(ctx context.Context, t domain.Track) error {
	if err := r.TrackRepository.Update(ctx, t); err != nil {
		return err
	}
	invalidate(ctx, r.c.Log, r.c.Tracks, t.ID)
	invalidate(ctx, r.c.Log, r.c.AlbumTracks, t.AlbumID)
	return nil
}

// Genres caches the curated genre list.
type Genres struct {
	next ports.GenreRepository
	c    Caches
}

// NewGenres wraps next.
func NewGenres(next ports.GenreRepository, c Caches) *Genres {
	return &Genres{next: next, c: c}
}

// List implements ports.GenreRepository.
func (r *Genres) List(ctx context.Context) ([]domain.Genre, error) {
	return r.c.Genres.GetOrLoad(ctx, genresKey, r.next.List)
}
