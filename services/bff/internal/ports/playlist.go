package ports

import (
	"context"
	"time"
)

// Playlist as read from the Playlist Service: tracks by ID, in play order.
type Playlist struct {
	ID        string
	OwnerID   string
	Title     string
	TrackIDs  []string
	UpdatedAt time.Time
}

// Playlists is the read side of the Playlist Service the BFF needs.
type Playlists interface {
	// GetPlaylist returns ErrNotFound for an unknown playlist.
	GetPlaylist(ctx context.Context, id string) (Playlist, error)
}

// Profiles resolves public display names of users.
type Profiles interface {
	// DisplayName returns ErrNotFound for an unknown user.
	DisplayName(ctx context.Context, userID string) (string, error)
}
