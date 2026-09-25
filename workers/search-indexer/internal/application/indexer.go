// Package application keeps the search indices in step with Catalog.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events/catalogv1"
	"github.com/tehrelt/icyre/libs/contracts/search"
)

// ErrNotFound means Catalog does not know a referenced entity.
var ErrNotFound = errors.New("catalog entity not found")

// Album, Artist and Track are what the indexer reads from Catalog.
type Album struct {
	ID          string
	Title       string
	AlbumType   string
	ReleaseDate string
	ArtistIDs   []string
	UpdatedAt   time.Time
}

type Artist struct {
	ID        string
	Name      string
	UpdatedAt time.Time
}

type Track struct {
	ID         string
	Title      string
	AlbumID    string
	ArtistIDs  []string
	DurationMs int64
	Explicit   bool
	Status     string
	UpdatedAt  time.Time
}

// Catalog is the source of truth used to denormalize documents and to
// rebuild the indices.
type Catalog interface {
	Album(ctx context.Context, id string) (Album, error)
	Artists(ctx context.Context, ids []string) (map[string]Artist, error)
	EachAlbum(ctx context.Context, fn func(Album) error) error
	AlbumTracks(ctx context.Context, albumID string) ([]Track, error)
}

// Write is one document change. Version orders writes: an older version
// never overwrites a newer document.
type Write struct {
	Index   string // alias or physical index
	ID      string
	Version int64
	Delete  bool
	Doc     any
}

// Index applies writes.
type Index interface {
	Apply(ctx context.Context, writes []Write) error
}

// Indexer implements the use cases.
type Indexer struct {
	catalog Catalog
	index   Index
	log     *slog.Logger
}

// New returns an Indexer.
func New(c Catalog, i Index, log *slog.Logger) *Indexer {
	return &Indexer{catalog: c, index: i, log: log}
}

// version turns a timestamp into an external document version.
func version(t time.Time) int64 { return t.UnixMilli() }

// TrackChanged handles track.created/track.updated. Tracks that are not
// searchable (draft, processing, deleted) are removed.
func (x *Indexer) TrackChanged(ctx context.Context, t catalogv1.Track, at time.Time) error {
	if !search.Searchable(t.Status) {
		return x.index.Apply(ctx, []Write{{Index: search.AliasTracks, ID: t.TrackID, Version: version(at), Delete: true}})
	}
	album, err := x.catalog.Album(ctx, t.AlbumID)
	if err != nil {
		return fmt.Errorf("album %s: %w", t.AlbumID, err)
	}
	artists, err := x.catalog.Artists(ctx, t.ArtistIDs)
	if err != nil {
		return err
	}
	doc := trackDoc(Track{ID: t.TrackID, Title: t.Title, AlbumID: t.AlbumID, ArtistIDs: t.ArtistIDs, DurationMs: t.DurationMs, Explicit: t.Explicit, Status: t.Status, UpdatedAt: t.UpdatedAt}, album, artists)
	return x.index.Apply(ctx, []Write{{Index: search.AliasTracks, ID: doc.ID, Version: version(at), Doc: doc}})
}

// AlbumCreated handles album.created.
func (x *Indexer) AlbumCreated(ctx context.Context, a catalogv1.Album, at time.Time) error {
	artists, err := x.catalog.Artists(ctx, a.ArtistIDs)
	if err != nil {
		return err
	}
	doc := albumDoc(Album{ID: a.AlbumID, Title: a.Title, AlbumType: a.AlbumType, ReleaseDate: a.ReleaseDate, ArtistIDs: a.ArtistIDs, UpdatedAt: at}, artists)
	return x.index.Apply(ctx, []Write{{Index: search.AliasAlbums, ID: doc.ID, Version: version(at), Doc: doc}})
}

// ArtistCreated handles artist.created.
func (x *Indexer) ArtistCreated(ctx context.Context, a catalogv1.Artist, at time.Time) error {
	doc := artistDoc(Artist{ID: a.ArtistID, Name: a.Name, UpdatedAt: at})
	return x.index.Apply(ctx, []Write{{Index: search.AliasArtists, ID: doc.ID, Version: version(at), Doc: doc}})
}

// batchSize bounds one bulk request during a rebuild.
const batchSize = 500

// Rebuild fills fresh indices (alias → physical index) from Catalog. The
// caller creates and promotes them (indices.Generation).
func (x *Indexer) Rebuild(ctx context.Context, target map[string]string) (Stats, error) {
	var st Stats
	var pending []Write
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		err := x.index.Apply(ctx, pending)
		pending = pending[:0]
		return err
	}
	add := func(w Write) error {
		pending = append(pending, w)
		if len(pending) >= batchSize {
			return flush()
		}
		return nil
	}

	seenArtists := map[string]bool{}
	var artistIDs []string
	err := x.catalog.EachAlbum(ctx, func(a Album) error {
		tracks, err := x.catalog.AlbumTracks(ctx, a.ID)
		if err != nil {
			return fmt.Errorf("tracks of %s: %w", a.ID, err)
		}
		ids := slices.Clone(a.ArtistIDs)
		for _, t := range tracks {
			ids = append(ids, t.ArtistIDs...)
		}
		artists, err := x.catalog.Artists(ctx, ids)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if !seenArtists[id] {
				seenArtists[id] = true
				artistIDs = append(artistIDs, id)
			}
		}
		album := albumDoc(a, artists)
		if err := add(Write{Index: target[search.AliasAlbums], ID: album.ID, Version: version(a.UpdatedAt), Doc: album}); err != nil {
			return err
		}
		st.Albums++
		for _, t := range tracks {
			if !search.Searchable(t.Status) {
				continue
			}
			doc := trackDoc(t, a, artists)
			if err := add(Write{Index: target[search.AliasTracks], ID: doc.ID, Version: version(t.UpdatedAt), Doc: doc}); err != nil {
				return err
			}
			st.Tracks++
		}
		return nil
	})
	if err != nil {
		return st, err
	}
	for chunk := range slices.Chunk(artistIDs, 100) {
		artists, err := x.catalog.Artists(ctx, chunk)
		if err != nil {
			return st, err
		}
		for _, id := range chunk {
			a, ok := artists[id]
			if !ok {
				continue
			}
			doc := artistDoc(a)
			if err := add(Write{Index: target[search.AliasArtists], ID: doc.ID, Version: version(a.UpdatedAt), Doc: doc}); err != nil {
				return st, err
			}
			st.Artists++
		}
	}
	return st, flush()
}

// Stats counts rebuilt documents.
type Stats struct{ Albums, Tracks, Artists int }

func names(ids []string, artists map[string]Artist) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if a, ok := artists[id]; ok {
			out = append(out, a.Name)
		}
	}
	return out
}

func trackDoc(t Track, a Album, artists map[string]Artist) search.Track {
	return search.Track{
		ID: t.ID, Title: t.Title, ArtistIDs: t.ArtistIDs, ArtistNames: names(t.ArtistIDs, artists),
		AlbumID: a.ID, AlbumTitle: a.Title, ReleaseDate: a.ReleaseDate,
		DurationMs: t.DurationMs, Explicit: t.Explicit, Available: t.Status == search.StatusReady,
		UpdatedAt: t.UpdatedAt.UTC(),
	}
}

func albumDoc(a Album, artists map[string]Artist) search.Album {
	return search.Album{
		ID: a.ID, Title: a.Title, AlbumType: a.AlbumType, ArtistIDs: a.ArtistIDs, ArtistNames: names(a.ArtistIDs, artists),
		ReleaseDate: a.ReleaseDate, UpdatedAt: a.UpdatedAt.UTC(),
	}
}

func artistDoc(a Artist) search.Artist {
	return search.Artist{ID: a.ID, Name: a.Name, UpdatedAt: a.UpdatedAt.UTC()}
}
