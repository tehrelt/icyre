// Package ports declares the upstream services the BFF aggregates.
// Types here are read models of those services, owned by the BFF — not
// shared DTOs and not imported from other services' internals.
package ports

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when an upstream resource does not exist.
var ErrNotFound = errors.New("not found")

// Artist as read from Catalog.
type Artist struct {
	ID   string
	Name string
}

// Album as read from Catalog.
type Album struct {
	ID          string
	Title       string
	AlbumType   string
	ReleaseDate time.Time
	ArtistIDs   []string
	GenreIDs    []string
}

// Track as read from Catalog.
type Track struct {
	ID          string
	AlbumID     string
	ArtistIDs   []string
	Title       string
	Duration    time.Duration
	TrackNumber int
	DiscNumber  int
	Explicit    bool
	Status      string
}

// Genre as read from Catalog.
type Genre struct {
	ID   string
	Slug string
	Name string
}

// Catalog is the read side of Catalog Service the BFF needs.
type Catalog interface {
	GetAlbum(ctx context.Context, id string) (Album, error)
	AlbumTracks(ctx context.Context, albumID string) ([]Track, error)
	// LatestAlbums returns the newest releases of the catalogue.
	LatestAlbums(ctx context.Context, limit int) ([]Album, error)
	ArtistAlbums(ctx context.Context, artistID string, limit int) ([]Album, error)
	// Artists resolves IDs in one batch; unknown IDs are skipped.
	Artists(ctx context.Context, ids []string) ([]Artist, error)
	Genres(ctx context.Context) ([]Genre, error)
}
