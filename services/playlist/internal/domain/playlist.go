// Package domain holds the Playlist aggregate.
package domain

import (
	"errors"
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
