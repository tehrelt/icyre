package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
	"github.com/tehrelt/icyre/services/bff/internal/views"
)

type fakeCatalog struct {
	albums       map[string]ports.Album
	tracks       map[string][]ports.Track
	artists      map[string]ports.Artist
	latest       []ports.Album
	failArtists  error
	failMore     error
	failTracks   error
	blockTracks  bool
	genreCalls   int
	artistsCalls int
}

func (f *fakeCatalog) GetAlbum(_ context.Context, id string) (ports.Album, error) {
	a, ok := f.albums[id]
	if !ok {
		return ports.Album{}, ports.ErrNotFound
	}
	return a, nil
}

func (f *fakeCatalog) AlbumTracks(ctx context.Context, id string) ([]ports.Track, error) {
	if f.blockTracks {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.tracks[id], f.failTracks
}

func (f *fakeCatalog) LatestAlbums(context.Context, int) ([]ports.Album, error) { return f.latest, nil }

func (f *fakeCatalog) ArtistAlbums(_ context.Context, artistID string, _ int) ([]ports.Album, error) {
	if f.failMore != nil {
		return nil, f.failMore
	}
	var out []ports.Album
	for _, a := range f.albums {
		if slices.Contains(a.ArtistIDs, artistID) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeCatalog) Artists(_ context.Context, ids []string) ([]ports.Artist, error) {
	f.artistsCalls++
	if f.failArtists != nil {
		return nil, f.failArtists
	}
	var out []ports.Artist
	for _, id := range ids {
		if a, ok := f.artists[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeCatalog) Genres(context.Context) ([]ports.Genre, error) {
	f.genreCalls++
	return []ports.Genre{{ID: "g1", Slug: "ambient-pop", Name: "Ambient pop"}}, nil
}

func (f *fakeCatalog) Tracks(_ context.Context, ids []string) ([]ports.Track, error) {
	if f.failTracks != nil {
		return nil, f.failTracks
	}
	var out []ports.Track
	for _, list := range f.tracks {
		for _, t := range list {
			if slices.Contains(ids, t.ID) {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func fixture() *fakeCatalog {
	day := func(y int) time.Time { return time.Date(y, 3, 6, 0, 0, 0, 0, time.UTC) }
	return &fakeCatalog{
		albums: map[string]ports.Album{
			"prism":  {ID: "prism", Title: "Prism Hours", AlbumType: "ALBUM", ReleaseDate: day(2026), ArtistIDs: []string{"nova"}, GenreIDs: []string{"g1"}},
			"winter": {ID: "winter", Title: "Winter Index", AlbumType: "EP", ReleaseDate: day(2024), ArtistIDs: []string{"nova"}},
		},
		tracks: map[string][]ports.Track{
			"prism": {
				{ID: "t1", AlbumID: "prism", ArtistIDs: []string{"nova"}, Title: "Frozen Choir", Duration: 240 * time.Second, Status: "READY"},
				{ID: "t2", AlbumID: "prism", ArtistIDs: []string{"nova", "kai"}, Title: "Mirror Weather", Duration: 198500 * time.Millisecond, Explicit: true, Status: "PROCESSING"},
			},
		},
		artists: map[string]ports.Artist{"nova": {ID: "nova", Name: "Nova Hale"}, "kai": {ID: "kai", Name: "Kai Frost"}},
	}
}

func newPages(c ports.Catalog) *Pages {
	return New(c, Personal{}, Config{PageBudget: 200 * time.Millisecond}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type fakeLibrary struct {
	saved map[string]bool
	err   error
}

func (f fakeLibrary) SavedTracks(ctx context.Context, ids []string) (map[string]bool, error) {
	if ports.UserToken(ctx) == "" {
		panic("library called without a user")
	}
	return f.saved, f.err
}

func TestAlbumPageMarksLikedTracks(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	lib := fakeLibrary{saved: map[string]bool{"t2": true}}
	p := New(fixture(), Personal{Library: lib}, Config{PageBudget: 200 * time.Millisecond}, quiet)

	page, err := p.Album(ports.WithUserToken(context.Background(), "tok"), "prism")
	if err != nil {
		t.Fatal(err)
	}
	if page.Tracks[0].Liked || !page.Tracks[1].Liked {
		t.Fatalf("liked marks: %+v", page.Tracks)
	}
	// Anonymous: Library is not called (fakeLibrary would panic).
	if page, _ := p.Album(context.Background(), "prism"); page.Tracks[1].Liked {
		t.Fatal("anonymous page has likes")
	}
	// Library down: the page still renders, marks degrade.
	p = New(fixture(), Personal{Library: fakeLibrary{err: errors.New("library 503")}}, Config{PageBudget: 200 * time.Millisecond}, quiet)
	page, err = p.Album(ports.WithUserToken(context.Background(), "tok"), "prism")
	if err != nil || page.Tracks[1].Liked || !slices.Contains(page.Unavailable, "liked") {
		t.Fatalf("degraded: %v %+v", err, page.Unavailable)
	}
}

func TestAlbumPageAggregates(t *testing.T) {
	cat := fixture()
	page, err := newPages(cat).Album(context.Background(), "prism")
	if err != nil {
		t.Fatal(err)
	}
	if page.Album.Title != "Prism Hours" || page.Album.Year != 2026 || page.Album.TrackCount != 2 || page.Album.DurationSec != 439 {
		t.Fatalf("header = %+v", page.Album)
	}
	if !slices.Equal(page.Album.Tags, []string{"Ambient pop"}) {
		t.Fatalf("tags = %v", page.Album.Tags)
	}
	if page.Artist.Name != "Nova Hale" {
		t.Fatalf("artist = %+v", page.Artist)
	}
	if got := page.Tracks[1]; got.ArtistName != "Nova Hale · Kai Frost" || got.Available || !got.Explicit || got.DurationSec != 199 {
		t.Fatalf("track = %+v", got)
	}
	if !page.Tracks[0].Available {
		t.Fatal("READY track must be available")
	}
	if len(page.MoreByArtist) != 1 || page.MoreByArtist[0].ID != "winter" || page.MoreByArtist[0].AlbumType != "EP" {
		t.Fatalf("more = %+v", page.MoreByArtist)
	}
	if len(page.Unavailable) != 0 {
		t.Fatalf("unexpected degradation: %v", page.Unavailable)
	}
	if cat.artistsCalls != 1 {
		t.Fatalf("artists must be resolved in one batch, got %d calls", cat.artistsCalls)
	}
}

func TestAlbumPageDegradesOptionalSections(t *testing.T) {
	cat := fixture()
	cat.failArtists = errors.New("catalog 503")
	cat.failMore = errors.New("timeout")
	page, err := newPages(cat).Album(context.Background(), "prism")
	if err != nil {
		t.Fatalf("optional failures must not fail the page: %v", err)
	}
	if page.Artist.Name != "Unknown artist" || len(page.MoreByArtist) != 0 {
		t.Fatalf("page = %+v", page)
	}
	if !slices.Contains(page.Unavailable, "artistNames") || !slices.Contains(page.Unavailable, "moreByArtist") {
		t.Fatalf("unavailable = %v", page.Unavailable)
	}
}

func TestAlbumPageRequiredFailures(t *testing.T) {
	if _, err := newPages(fixture()).Album(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing album: %v", err)
	}

	cat := fixture()
	cat.failTracks = errors.New("boom")
	if _, err := newPages(cat).Album(context.Background(), "prism"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("tracks failure: %v", err)
	}

	cat = fixture()
	cat.blockTracks = true
	start := time.Now()
	if _, err := newPages(cat).Album(context.Background(), "prism"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("budget: %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("page budget not enforced: %v", d)
	}
}

func TestGenresAreCached(t *testing.T) {
	cat := fixture()
	p := newPages(cat)
	for i := 0; i < 3; i++ {
		if _, err := p.Album(context.Background(), "prism"); err != nil {
			t.Fatal(err)
		}
	}
	if cat.genreCalls != 1 {
		t.Fatalf("genre calls = %d", cat.genreCalls)
	}
}

func TestHomeNewReleases(t *testing.T) {
	cat := fixture()
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	cat.latest = []ports.Album{cat.albums["prism"], cat.albums["winter"]}
	p := newPages(cat)
	p.now = func() time.Time { return now }

	page, err := p.Home(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items := page.NewReleases.Items
	if len(items) != 2 || items[0].ArtistName != "Nova Hale" || items[0].Badge != "new" || items[1].Badge != "" {
		t.Fatalf("new releases = %+v", items)
	}
	if !slices.Contains(page.Unavailable, "recommended") || slices.Contains(page.Unavailable, "newReleases") {
		t.Fatalf("unavailable = %v", page.Unavailable)
	}
	if page.RecentlyPlayed == nil || page.Trending.Today == nil {
		t.Fatal("empty sections must serialise as [] not null")
	}
}

func TestArtIndexIsStable(t *testing.T) {
	first := artIndex("prism")
	for i := 0; i < 3; i++ {
		if got := artIndex("prism"); got != first {
			t.Fatalf("art index changed: %d != %d", got, first)
		}
	}
	for _, id := range []string{"a", "b", "c", "0192-uuid"} {
		if n := artIndex(id); n < 0 || n > 7 {
			t.Fatalf("artIndex(%q) = %d, want 0..7", id, n)
		}
	}
}

type fakeHistory struct {
	sources []string
	err     error
}

func (f fakeHistory) RecentSources(context.Context, int) ([]string, error) { return f.sources, f.err }

func TestHomeRecentlyPlayed(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{PageBudget: 200 * time.Millisecond}
	hist := fakeHistory{sources: []string{"playlist:p1", "album:prism", "album:gone", "playlist:gone", "artist:nova"}}
	personal := Personal{History: hist, Playlists: fakePlaylists(), Profiles: fakeProfiles{"u1": "Mira"}}
	p := New(fixture(), personal, cfg, quiet)
	user := ports.WithUserToken(context.Background(), "tok")

	page, err := p.Home(user)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.RecentlyPlayed) != 2 || slices.Contains(page.Unavailable, "recentlyPlayed") {
		t.Fatalf("recent %+v unavailable %v", page.RecentlyPlayed, page.Unavailable)
	}
	// History order is kept: the playlist came first.
	pl := page.RecentlyPlayed[0].(views.PlaylistCard)
	if pl.Kind != "playlist" || pl.ID != "p1" || pl.Owner != "Mira" || pl.TrackCount != 3 {
		t.Fatalf("playlist card %+v", pl)
	}
	card := page.RecentlyPlayed[1].(views.AlbumCard)
	if card.ID != "prism" || card.ArtistName != "Nova Hale" || card.Kind != "album" {
		t.Fatalf("card %+v", card)
	}
	// Without the Playlist Service playlists are skipped, albums stay.
	p = New(fixture(), Personal{History: hist}, cfg, quiet)
	if page, _ := p.Home(user); len(page.RecentlyPlayed) != 1 {
		t.Fatalf("no playlists upstream %+v", page.RecentlyPlayed)
	}
	// Anonymous: empty, not degraded.
	if page, _ := p.Home(context.Background()); len(page.RecentlyPlayed) != 0 || slices.Contains(page.Unavailable, "recentlyPlayed") {
		t.Fatalf("anonymous %+v", page)
	}
	// History down: degraded, page still served.
	p = New(fixture(), Personal{History: fakeHistory{err: errors.New("503")}}, cfg, quiet)
	if page, err := p.Home(user); err != nil || !slices.Contains(page.Unavailable, "recentlyPlayed") {
		t.Fatalf("degraded: %v %v", err, page.Unavailable)
	}
}

type fakePlaylistsStore map[string]ports.Playlist

func fakePlaylists() fakePlaylistsStore {
	return fakePlaylistsStore{"p1": {ID: "p1", OwnerID: "u1", Title: "Late night", TrackIDs: []string{"t2", "gone", "t1"}, UpdatedAt: time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)}}
}

func (f fakePlaylistsStore) GetPlaylist(_ context.Context, id string) (ports.Playlist, error) {
	p, ok := f[id]
	if !ok {
		return ports.Playlist{}, ports.ErrNotFound
	}
	return p, nil
}

type fakeProfiles map[string]string

func (f fakeProfiles) DisplayName(_ context.Context, id string) (string, error) {
	if id == "down" {
		return "", errors.New("503")
	}
	n, ok := f[id]
	if !ok {
		return "", ports.ErrNotFound
	}
	return n, nil
}

func TestPlaylistPage(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{PageBudget: 200 * time.Millisecond}
	personal := Personal{Playlists: fakePlaylists(), Profiles: fakeProfiles{"u1": "Mira"}, Library: fakeLibrary{saved: map[string]bool{"t1": true}}}
	p := New(fixture(), personal, cfg, quiet)

	page, err := p.Playlist(ports.WithUserToken(context.Background(), "tok"), "p1")
	if err != nil {
		t.Fatal(err)
	}
	h := page.Playlist
	if h.Title != "Late night" || h.Owner != "Mira" || h.TrackCount != 2 || h.DurationSec != 439 || h.UpdatedAt != "2026-09-26T10:00:00Z" {
		t.Fatalf("header %+v", h)
	}
	// Playlist order, the track gone from Catalog left out, album and artists resolved.
	if len(page.Tracks) != 2 || page.Tracks[0].ID != "t2" || page.Tracks[1].ID != "t1" {
		t.Fatalf("tracks %+v", page.Tracks)
	}
	if tr := page.Tracks[0]; tr.AlbumTitle != "Prism Hours" || tr.ArtistName != "Nova Hale · Kai Frost" || tr.Available || tr.Liked {
		t.Fatalf("row %+v", tr)
	}
	if !page.Tracks[1].Liked || len(page.Unavailable) != 0 {
		t.Fatalf("liked / unavailable: %+v %v", page.Tracks[1], page.Unavailable)
	}

	if _, err := p.Playlist(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing playlist: %v", err)
	}
	cat := fixture()
	cat.failTracks = errors.New("catalog down")
	if _, err := New(cat, personal, cfg, quiet).Playlist(context.Background(), "p1"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("catalog down: %v", err)
	}
	if _, err := New(fixture(), Personal{}, cfg, quiet).Playlist(context.Background(), "p1"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("no playlist upstream: %v", err)
	}
	// Owner lookup failing degrades only the owner name.
	down := fakePlaylistsStore{"p1": {ID: "p1", OwnerID: "down", Title: "x"}}
	page, err = New(fixture(), Personal{Playlists: down, Profiles: fakeProfiles{}}, cfg, quiet).Playlist(context.Background(), "p1")
	if err != nil || page.Playlist.Owner != "" || !slices.Contains(page.Unavailable, "owners") {
		t.Fatalf("owner degraded: %v %+v", err, page)
	}
}
