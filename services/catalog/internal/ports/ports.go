// Package ports declares what the application layer needs from the outside
// world. Adapters (postgres, kafka) implement these interfaces.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

// ArtistRepository persists artists. Get returns domain.ErrArtistNotFound.
type ArtistRepository interface {
	Create(ctx context.Context, a domain.Artist) error
	Get(ctx context.Context, id uuid.UUID) (domain.Artist, error)
	// ListByIDs returns the artists that exist among ids, in no particular order.
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Artist, error)
}

// AlbumCursor positions keyset pagination over albums ordered by
// (release_date DESC, id DESC).
type AlbumCursor struct {
	ReleaseDate time.Time
	ID          uuid.UUID
}

// AlbumRepository persists albums with their artist and genre credits.
//
// Create returns a *domain.ReferenceError when a credited artist or genre
// does not exist. Get returns domain.ErrAlbumNotFound.
type AlbumRepository interface {
	Create(ctx context.Context, a domain.Album) error
	Get(ctx context.Context, id uuid.UUID) (domain.Album, error)
	// ListByArtist returns up to limit albums credited to the artist,
	// starting after the cursor (nil = first page).
	ListByArtist(ctx context.Context, artistID uuid.UUID, after *AlbumCursor, limit int) ([]domain.Album, error)
	// List returns up to limit albums of the whole catalogue, newest first.
	List(ctx context.Context, after *AlbumCursor, limit int) ([]domain.Album, error)
}

// TrackRepository persists tracks with their artist credits.
//
// Create returns domain.ErrTrackPositionTaken when the disc/track number is
// used on the album and a *domain.ReferenceError for missing artists.
type TrackRepository interface {
	Create(ctx context.Context, t domain.Track) error
	Get(ctx context.Context, id uuid.UUID) (domain.Track, error)
	Update(ctx context.Context, t domain.Track) error
	// ListByAlbum returns non-deleted tracks ordered by disc and track number.
	ListByAlbum(ctx context.Context, albumID uuid.UUID) ([]domain.Track, error)
	// ListByIDs returns the non-deleted tracks among ids, in no particular order.
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Track, error)
}

// GenreRepository reads the curated genre list.
type GenreRepository interface {
	List(ctx context.Context) ([]domain.Genre, error)
}

// EventPublisher delivers domain events to other services.
type EventPublisher interface {
	Publish(ctx context.Context, events ...domain.Event) error
}
