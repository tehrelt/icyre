// Package domain holds the Playlist aggregate.
package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Errors.
var (
	ErrNotFound      = errors.New("playlist not found")
	ErrForbidden     = errors.New("only the owner can change a playlist")
	ErrTrackNotFound = errors.New("track not found")
	ErrInvalidTitle  = errors.New("title must be 1–100 characters")
	// ErrOrderMismatch: a reorder must list exactly the playlist's tracks.
	ErrOrderMismatch = errors.New("order must list every playlist track exactly once")
)

// MaxTitle bounds a playlist title.
const MaxTitle = 100

// Playlist is an ordered list of tracks owned by one listener.
type Playlist struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Title      string
	TrackCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NormalizeTitle trims and validates a title.
func NormalizeTitle(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || utf8.RuneCountInString(s) > MaxTitle {
		return "", ErrInvalidTitle
	}
	return s, nil
}

// Track is a PlaylistTrack: a track at a position.
type Track struct {
	TrackID  uuid.UUID
	Position int
	AddedBy  uuid.UUID
	AddedAt  time.Time
}

// CheckOrder reports whether order lists exactly the tracks of current, each
// once (ErrOrderMismatch otherwise).
func CheckOrder(current []Track, order []uuid.UUID) error {
	if len(order) != len(current) {
		return ErrOrderMismatch
	}
	seen := make(map[uuid.UUID]bool, len(order))
	for _, id := range order {
		if seen[id] || !slices.ContainsFunc(current, func(t Track) bool { return t.TrackID == id }) {
			return ErrOrderMismatch
		}
		seen[id] = true
	}
	return nil
}
