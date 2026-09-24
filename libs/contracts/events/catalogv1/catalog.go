// Package catalogv1 holds version 1 payloads of Catalog Service events,
// published to events.TopicCatalogEvents with the aggregate ID as key.
package catalogv1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types.
const (
	TypeTrackCreated  = "track.created"
	TypeTrackUpdated  = "track.updated"
	TypeAlbumCreated  = "album.created"
	TypeArtistCreated = "artist.created"
)

// Track is the payload of track.created and track.updated: the full state
// of the track after the change, so consumers never need a read-back.
type Track struct {
	TrackID     string    `json:"trackId"`
	AlbumID     string    `json:"albumId"`
	ArtistIDs   []string  `json:"artistIds"`
	Title       string    `json:"title"`
	DurationMs  int64     `json:"durationMs"`
	TrackNumber int       `json:"trackNumber"`
	DiscNumber  int       `json:"discNumber"`
	Explicit    bool      `json:"explicit"`
	ISRC        string    `json:"isrc,omitempty"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Album is the payload of album.created.
type Album struct {
	AlbumID     string   `json:"albumId"`
	ArtistIDs   []string `json:"artistIds"`
	Title       string   `json:"title"`
	AlbumType   string   `json:"albumType"`
	ReleaseDate string   `json:"releaseDate,omitempty"` // YYYY-MM-DD
	GenreIDs    []string `json:"genreIds"`
}

// Artist is the payload of artist.created.
type Artist struct {
	ArtistID string `json:"artistId"`
	Name     string `json:"name"`
}
