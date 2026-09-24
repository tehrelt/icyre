package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// AlbumType classifies a release.
type AlbumType string

// Release types.
const (
	AlbumTypeAlbum       AlbumType = "ALBUM"
	AlbumTypeEP          AlbumType = "EP"
	AlbumTypeSingle      AlbumType = "SINGLE"
	AlbumTypeCompilation AlbumType = "COMPILATION"
)

// Valid reports whether t is a known album type.
func (t AlbumType) Valid() bool {
	switch t {
	case AlbumTypeAlbum, AlbumTypeEP, AlbumTypeSingle, AlbumTypeCompilation:
		return true
	}
	return false
}

// MaxCredits bounds artists and genres attached to one album or track.
const MaxCredits = 20

// Album is a release: a titled, dated set of tracks by one or more artists.
type Album struct {
	ID          uuid.UUID
	Title       string
	Type        AlbumType
	ReleaseDate time.Time // calendar date, UTC midnight
	ArtistIDs   []uuid.UUID
	GenreIDs    []uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewAlbumParams are the inputs of NewAlbum.
type NewAlbumParams struct {
	ID          uuid.UUID
	Title       string
	Type        AlbumType
	ReleaseDate time.Time
	ArtistIDs   []uuid.UUID
	GenreIDs    []uuid.UUID
}

// NewAlbum validates and creates an album.
func NewAlbum(p NewAlbumParams, now time.Time) (Album, error) {
	title := strings.TrimSpace(p.Title)
	artists := dedupe(p.ArtistIDs)
	genres := dedupe(p.GenreIDs)

	v := validator{}
	v.check(p.ID != uuid.Nil, "id", "must be set")
	v.check(title != "", "title", "must not be empty")
	v.check(utf8.RuneCountInString(title) <= MaxNameLength, "title", "is too long")
	v.check(p.Type.Valid(), "albumType", "must be one of ALBUM, EP, SINGLE, COMPILATION")
	v.check(!p.ReleaseDate.IsZero(), "releaseDate", "must be set")
	v.check(len(artists) > 0, "artistIds", "must contain at least one artist")
	v.check(len(artists) <= MaxCredits, "artistIds", "has too many artists")
	v.check(!containsNil(artists), "artistIds", "must not contain empty IDs")
	v.check(len(genres) <= MaxCredits, "genreIds", "has too many genres")
	v.check(!containsNil(genres), "genreIds", "must not contain empty IDs")
	if err := v.err(); err != nil {
		return Album{}, err
	}

	now = now.UTC()
	d := p.ReleaseDate.UTC()
	return Album{
		ID:          p.ID,
		Title:       title,
		Type:        p.Type,
		ReleaseDate: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC),
		ArtistIDs:   artists,
		GenreIDs:    genres,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// dedupe removes duplicates while keeping the first-seen (credit) order.
func dedupe(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func containsNil(ids []uuid.UUID) bool {
	for _, id := range ids {
		if id == uuid.Nil {
			return true
		}
	}
	return false
}
