// Package domain holds the playback authorization rules: which tracks may be
// streamed and which audio variant a listener gets.
package domain

import (
	"errors"
	"slices"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
)

// Errors. Each maps to a stable API code.
var (
	ErrTrackNotFound  = errors.New("track not found")
	ErrTrackNotReady  = errors.New("track is not ready for playback")
	ErrTrackBlocked   = errors.New("track is blocked")
	ErrNoVariant      = errors.New("no audio variant is available")
	ErrInvalidRequest = errors.New("invalid request")
)

// Track statuses as published by Catalog.
const (
	StatusDraft      = "DRAFT"
	StatusProcessing = "PROCESSING"
	StatusReady      = "READY"
	StatusBlocked    = "BLOCKED"
	StatusDeleted    = "DELETED"
)

// DefaultQuality is used when the client does not ask for one.
const DefaultQuality = media.Quality256

// CheckPlayable applies the track-status rule: only READY tracks stream.
// Deleted tracks look like missing ones.
func CheckPlayable(status string) error {
	switch status {
	case StatusReady:
		return nil
	case StatusBlocked:
		return ErrTrackBlocked
	case StatusDeleted:
		return ErrTrackNotFound
	default:
		return ErrTrackNotReady
	}
}

// ChooseVariant picks the requested quality when it exists, otherwise the
// best lower one, otherwise the lowest higher one.
func ChooseVariant(requested media.Quality, available []media.Quality) (media.Quality, error) {
	if len(available) == 0 {
		return 0, ErrNoVariant
	}
	sorted := slices.Clone(available)
	slices.Sort(sorted)
	best := media.Quality(0)
	for _, q := range sorted {
		if q <= requested {
			best = q
		}
	}
	if best != 0 {
		return best, nil
	}
	return sorted[0], nil
}

// Grant is an issued stream URL. It carries no user data: the URL is scoped
// to one immutable object and expires on its own.
type Grant struct {
	TrackID   string
	Quality   media.Quality
	URL       string
	ExpiresAt time.Time
}
