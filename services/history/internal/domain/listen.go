// Package domain holds the listening history: plays that count as listens.
package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Listen is one completed listen of a track.
type Listen struct {
	PlaybackID uuid.UUID // idempotency key: one play of one track
	UserID     uuid.UUID
	TrackID    uuid.UUID
	Source     string // e.g. "album:<id>", may be empty
	DurationMs int64
	ListenedMs int64
	PlayedAt   time.Time
}

// Rule decides whether a play counts as a listen (specs/workers/
// listening-history.md): at least MinListen, or at least MinFraction of the
// track. Both thresholds are business configuration.
type Rule struct {
	MinListen   time.Duration
	MinFraction float64
}

// DefaultRule is 30 seconds or half the track, whichever comes first.
var DefaultRule = Rule{MinListen: 30 * time.Second, MinFraction: 0.5}

// Counts reports whether a play of listenedMs out of durationMs is a listen.
func (r Rule) Counts(listenedMs, durationMs int64) bool {
	if listenedMs <= 0 {
		return false
	}
	if time.Duration(listenedMs)*time.Millisecond >= r.MinListen {
		return true
	}
	return durationMs > 0 && float64(listenedMs) >= r.MinFraction*float64(durationMs)
}

// RecentSource is a collection the listener played from, most recent first.
type RecentSource struct {
	Source   string
	PlayedAt time.Time
}

// ErrInvalidCursor rejects a malformed page cursor.
var ErrInvalidCursor = errors.New("invalid cursor")

// Cursor is a keyset position in the newest-first history.
type Cursor struct {
	PlayedAt   time.Time `json:"t"`
	PlaybackID uuid.UUID `json:"i"`
}

// Encode renders an opaque cursor.
func (c Cursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor parses Encode's output.
func DecodeCursor(s string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil || c.PlaybackID == uuid.Nil || c.PlayedAt.IsZero() {
		return Cursor{}, ErrInvalidCursor
	}
	return c, nil
}
