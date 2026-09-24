package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxNameLength bounds artist names and album/track titles (in runes).
const MaxNameLength = 200

// Artist is a performer credited on albums and tracks.
type Artist struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewArtist validates and creates an artist.
func NewArtist(id uuid.UUID, name string, now time.Time) (Artist, error) {
	name = strings.TrimSpace(name)
	v := validator{}
	v.check(id != uuid.Nil, "id", "must be set")
	v.check(name != "", "name", "must not be empty")
	v.check(utf8.RuneCountInString(name) <= MaxNameLength, "name", "is too long")
	if err := v.err(); err != nil {
		return Artist{}, err
	}
	now = now.UTC()
	return Artist{ID: id, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}

// Genre is a curated catalog classification.
type Genre struct {
	ID   uuid.UUID
	Slug string
	Name string
}
