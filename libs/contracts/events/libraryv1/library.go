// Package libraryv1 holds version 1 payloads of Library Service events,
// published to events.TopicLibraryEvents keyed by user ID (per-user order).
package libraryv1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types. Emitted only when the library actually changed: saving an
// already saved item or removing a missing one publishes nothing.
const (
	TypeTrackSaved   = "library.track_saved"
	TypeTrackRemoved = "library.track_removed"
	TypeAlbumSaved   = "library.album_saved"
	TypeAlbumRemoved = "library.album_removed"
)

// TrackSaved is the payload of library.track_saved.
type TrackSaved struct {
	UserID  string    `json:"userId"`
	TrackID string    `json:"trackId"`
	SavedAt time.Time `json:"savedAt"`
}

// TrackRemoved is the payload of library.track_removed.
type TrackRemoved struct {
	UserID    string    `json:"userId"`
	TrackID   string    `json:"trackId"`
	RemovedAt time.Time `json:"removedAt"`
}

// AlbumSaved is the payload of library.album_saved.
type AlbumSaved struct {
	UserID  string    `json:"userId"`
	AlbumID string    `json:"albumId"`
	SavedAt time.Time `json:"savedAt"`
}

// AlbumRemoved is the payload of library.album_removed.
type AlbumRemoved struct {
	UserID    string    `json:"userId"`
	AlbumID   string    `json:"albumId"`
	RemovedAt time.Time `json:"removedAt"`
}
