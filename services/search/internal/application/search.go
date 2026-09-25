// Package application implements full-text search and autocomplete over
// the derived OpenSearch indices (the service never reads PostgreSQL).
package application

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/tehrelt/icyre/libs/contracts/search"
)

// Type filters results by entity.
type Type string

// Types.
const (
	TypeAll       Type = "all"
	TypeTracks    Type = "tracks"
	TypeArtists   Type = "artists"
	TypeAlbums    Type = "albums"
	TypePlaylists Type = "playlists"
)

// ParseType accepts all|tracks|artists|albums|playlists ("" = all).
func ParseType(s string) (Type, bool) {
	switch t := Type(s); t {
	case "":
		return TypeAll, true
	case TypeAll, TypeTracks, TypeArtists, TypeAlbums, TypePlaylists:
		return t, true
	}
	return "", false
}

// Limits.
const (
	MaxQueryLength = 200
	MaxLimit       = 50
)

// ErrInvalidQuery rejects empty or oversized queries.
var ErrInvalidQuery = errors.New("invalid query")

// Sizes asks the engine for this many hits per index.
type Sizes struct{ Tracks, Artists, Albums, Playlists int }

// Hit is a document with its relevance score.
type Hit[T any] struct {
	Doc   T
	Score float64
}

// Page is the engine's answer: hits and total matches per index.
type Page struct {
	Tracks    []Hit[search.Track]
	Artists   []Hit[search.Artist]
	Albums    []Hit[search.Album]
	Playlists []Hit[search.Playlist]
	Counts    Counts
}

// Counts are total matches per entity type.
type Counts struct{ Tracks, Artists, Albums, Playlists int }

func (c Counts) total() int { return c.Tracks + c.Artists + c.Albums + c.Playlists }

// Suggestion is one autocomplete entry.
type Suggestion struct {
	Kind     string // track | artist | album | playlist
	ID       string
	Text     string
	Subtitle string
	Score    float64
}

// Engine runs queries against the index.
type Engine interface {
	Search(ctx context.Context, q string, sizes Sizes) (Page, error)
	Suggest(ctx context.Context, q string, limit int) ([]Suggestion, error)
	// Correct returns a spelling correction of q, or "" when there is none.
	Correct(ctx context.Context, q string) (string, error)
}

// TopKind says which collection is the top result.
type TopKind string

// Top result kinds.
const (
	TopArtist   TopKind = "artist"
	TopAlbum    TopKind = "album"
	TopPlaylist TopKind = "playlist"
)

// Result is a search answer.
type Result struct {
	Query      string
	Type       Type
	Page       Page
	TopKind    TopKind // "" when there is no top result
	DidYouMean string  // "" when not needed
}

// Service implements the use cases.
type Service struct{ engine Engine }

// New returns a Service.
func New(e Engine) *Service { return &Service{engine: e} }

// Normalize trims and validates a query.
func Normalize(q string) (string, error) {
	q = strings.Join(strings.Fields(q), " ")
	if q == "" || utf8.RuneCountInString(q) > MaxQueryLength {
		return "", ErrInvalidQuery
	}
	return q, nil
}

// sizes: the "all" tab shows a few of each kind (canvas Search results);
// a type tab shows up to limit of one kind. Counts are always complete.
func sizes(t Type, limit int) Sizes {
	if t == TypeAll {
		return Sizes{Tracks: min(4, limit), Artists: min(6, limit), Albums: min(6, limit), Playlists: min(6, limit)}
	}
	var s Sizes
	switch t {
	case TypeTracks:
		s.Tracks = limit
	case TypeArtists:
		s.Artists = limit
	case TypeAlbums:
		s.Albums = limit
	case TypePlaylists:
		s.Playlists = limit
	}
	return s
}

// Search runs a full-text query.
func (s *Service) Search(ctx context.Context, rawQuery string, t Type, limit int) (Result, error) {
	q, err := Normalize(rawQuery)
	if err != nil {
		return Result{}, err
	}
	limit = max(1, min(limit, MaxLimit))
	page, err := s.engine.Search(ctx, q, sizes(t, limit))
	if err != nil {
		return Result{}, err
	}
	res := Result{Query: q, Type: t, Page: page}
	if t == TypeAll {
		res.TopKind = topResult(q, page)
	}
	if page.Counts.total() == 0 {
		// Best effort: a failed correction must not fail the search.
		if c, err := s.engine.Correct(ctx, q); err == nil && !strings.EqualFold(c, q) {
			res.DidYouMean = c
		}
	}
	return res, nil
}

// Suggest returns autocomplete entries.
func (s *Service) Suggest(ctx context.Context, rawQuery string, limit int) ([]Suggestion, error) {
	q, err := Normalize(rawQuery)
	if err != nil {
		return nil, err
	}
	return s.engine.Suggest(ctx, q, max(1, min(limit, 20)))
}

// topResult picks the collection shown large: an artist whose name matches
// the query exactly or as a prefix, otherwise such an album, otherwise the
// best-scoring first artist, album or playlist.
func topResult(q string, p Page) TopKind {
	lq := strings.ToLower(q)
	strong := func(s string) bool { return strings.HasPrefix(strings.ToLower(s), lq) }
	if len(p.Artists) > 0 && strong(p.Artists[0].Doc.Name) {
		return TopArtist
	}
	if len(p.Albums) > 0 && strong(p.Albums[0].Doc.Title) {
		return TopAlbum
	}
	best, kind := -1.0, TopKind("")
	if len(p.Artists) > 0 && p.Artists[0].Score > best {
		best, kind = p.Artists[0].Score, TopArtist
	}
	if len(p.Albums) > 0 && p.Albums[0].Score > best {
		best, kind = p.Albums[0].Score, TopAlbum
	}
	if len(p.Playlists) > 0 && p.Playlists[0].Score > best {
		kind = TopPlaylist
	}
	return kind
}
