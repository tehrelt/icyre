// Package search is the contract between the Search Indexer (writes) and
// the Search Service (reads): index aliases and document shapes in
// OpenSearch. OpenSearch is a derived index (ADR-0004) — every document can
// be rebuilt from Catalog.
package search

import "time"

// Aliases the services read and write. Physical indices are versioned
// (<alias>-v<N>) so a rebuild can swap them without downtime.
const (
	AliasTracks    = "tracks"
	AliasAlbums    = "albums"
	AliasArtists   = "artists"
	AliasPlaylists = "playlists"
)

// Aliases lists every search alias.
var Aliases = []string{AliasTracks, AliasAlbums, AliasArtists, AliasPlaylists}

// Track statuses that are searchable. Other tracks are removed from the
// index; BLOCKED ones stay visible but unplayable (Available=false).
const (
	StatusReady   = "READY"
	StatusBlocked = "BLOCKED"
)

// Searchable reports whether a track in this status belongs in the index.
func Searchable(status string) bool { return status == StatusReady || status == StatusBlocked }

// Track is a document in the tracks index, denormalized for display.
type Track struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	ArtistIDs   []string  `json:"artistIds"`
	ArtistNames []string  `json:"artistNames"`
	AlbumID     string    `json:"albumId"`
	AlbumTitle  string    `json:"albumTitle"`
	ReleaseDate string    `json:"releaseDate,omitempty"` // album's, YYYY-MM-DD
	DurationMs  int64     `json:"durationMs"`
	Explicit    bool      `json:"explicit"`
	Available   bool      `json:"available"`
	Popularity  float64   `json:"popularity"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Album is a document in the albums index.
type Album struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	AlbumType   string    `json:"albumType"`
	ArtistIDs   []string  `json:"artistIds"`
	ArtistNames []string  `json:"artistNames"`
	ReleaseDate string    `json:"releaseDate,omitempty"`
	Popularity  float64   `json:"popularity"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Artist is a document in the artists index.
type Artist struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Verified   bool      `json:"verified"`
	Popularity float64   `json:"popularity"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Playlist is a document in the playlists index (filled once the Playlist
// Service exists, EPIC-013).
type Playlist struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	OwnerName  string    `json:"ownerName"`
	TrackCount int       `json:"trackCount"`
	Popularity float64   `json:"popularity"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
