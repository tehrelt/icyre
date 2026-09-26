// Package playlistv1 holds version 1 payloads of Playlist Service events,
// published to events.TopicPlaylistEvents keyed by playlist ID (per-playlist
// order).
package playlistv1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types. Emitted only when the playlist actually changed: adding a
// track that is already there or removing a missing one publishes nothing.
const (
	TypeCreated         = "playlist.created"
	TypeUpdated         = "playlist.updated"
	TypeDeleted         = "playlist.deleted"
	TypeTrackAdded      = "playlist.track_added"
	TypeTrackRemoved    = "playlist.track_removed"
	TypeTracksReordered = "playlist.tracks_reordered"
)

// Created is the payload of playlist.created.
type Created struct {
	PlaylistID string    `json:"playlistId"`
	OwnerID    string    `json:"ownerId"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Updated is the payload of playlist.updated (title changed).
type Updated struct {
	PlaylistID string    `json:"playlistId"`
	OwnerID    string    `json:"ownerId"`
	Title      string    `json:"title"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Deleted is the payload of playlist.deleted.
type Deleted struct {
	PlaylistID string    `json:"playlistId"`
	OwnerID    string    `json:"ownerId"`
	DeletedAt  time.Time `json:"deletedAt"`
}

// TrackAdded is the payload of playlist.track_added.
type TrackAdded struct {
	PlaylistID string    `json:"playlistId"`
	OwnerID    string    `json:"ownerId"`
	TrackID    string    `json:"trackId"`
	Position   int       `json:"position"`
	AddedAt    time.Time `json:"addedAt"`
}

// TrackRemoved is the payload of playlist.track_removed.
type TrackRemoved struct {
	PlaylistID string    `json:"playlistId"`
	OwnerID    string    `json:"ownerId"`
	TrackID    string    `json:"trackId"`
	RemovedAt  time.Time `json:"removedAt"`
}

// TracksReordered is the payload of playlist.tracks_reordered: the complete
// new order of track IDs.
type TracksReordered struct {
	PlaylistID  string    `json:"playlistId"`
	OwnerID     string    `json:"ownerId"`
	TrackIDs    []string  `json:"trackIds"`
	ReorderedAt time.Time `json:"reorderedAt"`
}
