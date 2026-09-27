// Package domain holds the social graph: listeners following users and artists.
package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// TargetType is what a follow points at.
type TargetType string

// Target types.
const (
	TargetUser   TargetType = "user"
	TargetArtist TargetType = "artist"
)

// ParseTargetType accepts "user" and "artist".
func ParseTargetType(s string) (TargetType, bool) {
	switch t := TargetType(s); t {
	case TargetUser, TargetArtist:
		return t, true
	}
	return "", false
}

// Errors.
var (
	ErrNotFound      = errors.New("follow target not found")
	ErrSelfFollow    = errors.New("cannot follow yourself")
	ErrInvalidCursor = errors.New("invalid cursor")
)

// Target is a followed user or artist.
type Target struct {
	Type TargetType
	ID   uuid.UUID
}

// Follow is one edge of the graph.
type Follow struct {
	FollowerID uuid.UUID
	Target     Target
	FollowedAt time.Time
}

// Change says whether a follow/unfollow altered the graph. Following twice or
// unfollowing a target that is not followed is a successful no-op
// (idempotent PUT/DELETE). For an unfollow Follow.FollowedAt is the removal time.
type Change struct {
	Follow  Follow
	Changed bool
}

// Cursor is a keyset position in a newest-first list; ID is the other end of
// the edge (the follower in a followers list, the target in a following list).
type Cursor struct {
	At time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
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
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == uuid.Nil || c.At.IsZero() {
		return Cursor{}, ErrInvalidCursor
	}
	return c, nil
}

// Page limits.
const (
	DefaultLimit = 50
	MaxLimit     = 100
	// MaxContains bounds one "do I follow these?" lookup.
	MaxContains = 100
)

// Counts are the follow counters of a user or an artist (artists only have
// followers).
type Counts struct {
	Followers        int64
	FollowingUsers   int64
	FollowingArtists int64
}
