// Package domain holds the listener's library: saved tracks and albums.
package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Kind is what a library item refers to.
type Kind string

// Kinds.
const (
	KindTrack Kind = "track"
	KindAlbum Kind = "album"
)

// Errors.
var (
	ErrNotFound      = errors.New("catalog item not found")
	ErrInvalidCursor = errors.New("invalid cursor")
)

// Item is one saved entry.
type Item struct {
	UserID   uuid.UUID
	Kind     Kind
	EntityID uuid.UUID
	SavedAt  time.Time
}

// Change says whether a save/remove altered the library. Saving an item
// twice or removing a missing one is a successful no-op (idempotent PUT/DELETE).
type Change struct {
	Item    Item
	Changed bool
}

// Cursor is a keyset position in a newest-first list.
type Cursor struct {
	SavedAt  time.Time `json:"t"`
	EntityID uuid.UUID `json:"i"`
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
	if err := json.Unmarshal(raw, &c); err != nil || c.EntityID == uuid.Nil || c.SavedAt.IsZero() {
		return Cursor{}, ErrInvalidCursor
	}
	return c, nil
}

// Page limits.
const (
	DefaultLimit = 50
	MaxLimit     = 100
	// MaxContains bounds one "is it saved?" lookup (a page of tracks).
	MaxContains = 100
)

// Counts summarises a library.
type Counts struct {
	Tracks int
	Albums int
}
