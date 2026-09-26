package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events/catalogv1"
	"github.com/tehrelt/icyre/libs/contracts/search"
)

type fakeCatalog struct {
	albums  []Album
	artists map[string]Artist
	tracks  map[string][]Track
}

func (f fakeCatalog) Album(_ context.Context, id string) (Album, error) {
	for _, a := range f.albums {
		if a.ID == id {
			return a, nil
		}
	}
	return Album{}, ErrNotFound
}
func (f fakeCatalog) Artists(_ context.Context, ids []string) (map[string]Artist, error) {
	out := map[string]Artist{}
	for _, id := range ids {
		if a, ok := f.artists[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}
func (f fakeCatalog) EachAlbum(_ context.Context, fn func(Album) error) error {
	for _, a := range f.albums {
		if err := fn(a); err != nil {
			return err
		}
	}
	return nil
}
func (f fakeCatalog) AlbumTracks(_ context.Context, id string) ([]Track, error) {
	return f.tracks[id], nil
}

type recorder struct {
	writes []Write
	calls  int
}

func (r *recorder) Apply(_ context.Context, w []Write) error {
	r.calls++
	r.writes = append(r.writes, w...)
	return nil
}

var (
	t0      = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	catalog = fakeCatalog{
		albums:  []Album{{ID: "al1", Title: "Prism Hours", AlbumType: "ALBUM", ReleaseDate: "2026-03-06", ArtistIDs: []string{"ar1"}, UpdatedAt: t0}},
		artists: map[string]Artist{"ar1": {ID: "ar1", Name: "Nova Hale", UpdatedAt: t0}, "ar2": {ID: "ar2", Name: "Kai Frost", UpdatedAt: t0}},
		tracks: map[string][]Track{"al1": {
			{ID: "t1", Title: "Glass Tides", AlbumID: "al1", ArtistIDs: []string{"ar1"}, Status: "READY", UpdatedAt: t0},
			{ID: "t2", Title: "Mirror Weather", AlbumID: "al1", ArtistIDs: []string{"ar1", "ar2"}, Status: "BLOCKED", UpdatedAt: t0},
			{ID: "t3", Title: "Draft", AlbumID: "al1", ArtistIDs: []string{"ar1"}, Status: "DRAFT", UpdatedAt: t0},
		}},
	}
	playlists = fakePlaylists{"p1": {ID: "p1", OwnerID: "u1", Title: "Late night", TrackCount: 4, UpdatedAt: t0}, "p2": {ID: "p2", OwnerID: "ghost", Title: "Orphan", UpdatedAt: t0}}
	profiles  = fakeProfiles{"u1": "Nova"}
	quiet     = slog.New(slog.NewTextHandler(io.Discard, nil))
)

type fakePlaylists map[string]Playlist

func (f fakePlaylists) Playlist(_ context.Context, id string) (Playlist, error) {
	p, ok := f[id]
	if !ok {
		return Playlist{}, ErrNotFound
	}
	return p, nil
}
func (f fakePlaylists) EachPlaylist(_ context.Context, fn func(Playlist) error) error {
	for _, id := range []string{"p1", "p2"} {
		if err := fn(f[id]); err != nil {
			return err
		}
	}
	return nil
}

type fakeProfiles map[string]string

func (f fakeProfiles) DisplayNames(_ context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if n, ok := f[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func TestPlaylistEvents(t *testing.T) {
	rec := &recorder{}
	x := New(catalog, playlists, profiles, rec, quiet)
	ctx := context.Background()
	at := t0.Add(time.Hour)
	if err := x.PlaylistChanged(ctx, "p1", at); err != nil {
		t.Fatal(err)
	}
	w := rec.writes[0]
	if doc := w.Doc.(search.Playlist); w.Index != search.AliasPlaylists || w.Version != at.UnixMilli() || doc.OwnerName != "Nova" || doc.TrackCount != 4 {
		t.Fatalf("write %+v", w)
	}
	// Gone from the Playlist Service by the time the event is handled: removed.
	if err := x.PlaylistChanged(ctx, "gone", at); err != nil {
		t.Fatal(err)
	}
	if err := x.PlaylistDeleted(ctx, "p1", at); err != nil {
		t.Fatal(err)
	}
	for _, w := range rec.writes[1:] {
		if !w.Delete || w.Version != at.UnixMilli() {
			t.Fatalf("delete %+v", w)
		}
	}
}

func TestTrackChanged(t *testing.T) {
	rec := &recorder{}
	x := New(catalog, playlists, profiles, rec, quiet)
	ctx := context.Background()
	at := t0.Add(time.Minute)

	err := x.TrackChanged(ctx, catalogv1.Track{TrackID: "t2", AlbumID: "al1", ArtistIDs: []string{"ar1", "ar2"}, Title: "Mirror Weather", Status: "BLOCKED", Explicit: true, DurationMs: 198000, UpdatedAt: at}, at)
	if err != nil {
		t.Fatal(err)
	}
	w := rec.writes[0]
	doc := w.Doc.(search.Track)
	if w.Index != search.AliasTracks || w.Version != at.UnixMilli() || doc.Available || doc.AlbumTitle != "Prism Hours" ||
		len(doc.ArtistNames) != 2 || doc.ArtistNames[1] != "Kai Frost" || doc.ReleaseDate != "2026-03-06" {
		t.Fatalf("write %+v doc %+v", w, doc)
	}

	if err := x.TrackChanged(ctx, catalogv1.Track{TrackID: "t9", AlbumID: "al1", Status: "DELETED"}, at); err != nil {
		t.Fatal(err)
	}
	if last := rec.writes[len(rec.writes)-1]; !last.Delete || last.ID != "t9" || last.Version != at.UnixMilli() {
		t.Fatalf("delete %+v", last)
	}

	if err := x.TrackChanged(ctx, catalogv1.Track{TrackID: "t8", AlbumID: "missing", Status: "READY"}, at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing album: %v", err)
	}
}

func TestCreatedEvents(t *testing.T) {
	rec := &recorder{}
	x := New(catalog, playlists, profiles, rec, quiet)
	ctx := context.Background()
	_ = x.AlbumCreated(ctx, catalogv1.Album{AlbumID: "al2", Title: "Hollow Signal", AlbumType: "ALBUM", ArtistIDs: []string{"ar2"}}, t0)
	_ = x.ArtistCreated(ctx, catalogv1.Artist{ArtistID: "ar3", Name: "Mira Solen"}, t0)
	if a := rec.writes[0].Doc.(search.Album); a.ArtistNames[0] != "Kai Frost" || rec.writes[0].Index != search.AliasAlbums {
		t.Fatalf("album %+v", a)
	}
	if a := rec.writes[1].Doc.(search.Artist); a.Name != "Mira Solen" || rec.writes[1].Version != t0.UnixMilli() {
		t.Fatalf("artist %+v", a)
	}
}

func TestRebuild(t *testing.T) {
	rec := &recorder{}
	x := New(catalog, playlists, profiles, rec, quiet)
	target := map[string]string{search.AliasTracks: "tracks-v2", search.AliasAlbums: "albums-v2", search.AliasArtists: "artists-v2", search.AliasPlaylists: "playlists-v2"}
	st, err := x.Rebuild(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if st != (Stats{Albums: 1, Tracks: 2, Artists: 2, Playlists: 2}) {
		t.Fatalf("stats %+v", st)
	}
	byIndex := map[string]int{}
	for _, w := range rec.writes {
		byIndex[w.Index]++
	}
	if byIndex["tracks-v2"] != 2 || byIndex["albums-v2"] != 1 || byIndex["artists-v2"] != 2 || byIndex["playlists-v2"] != 2 || byIndex[search.AliasTracks] != 0 {
		t.Fatalf("writes %v", byIndex)
	}
}
