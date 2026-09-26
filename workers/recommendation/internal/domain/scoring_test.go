package domain

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

var now = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

// fixture: two artists in different genres; the user loves artist A.
type fixture struct {
	artistA, artistB, rock, jazz, albumA1, albumA2, albumB uuid.UUID
	a1, a2, a3, b1, b2                                     uuid.UUID
	cat                                                    *Catalog
}

func newFixture() fixture {
	f := fixture{
		artistA: uuid.New(), artistB: uuid.New(), rock: uuid.New(), jazz: uuid.New(),
		albumA1: uuid.New(), albumA2: uuid.New(), albumB: uuid.New(),
		a1: uuid.New(), a2: uuid.New(), a3: uuid.New(), b1: uuid.New(), b2: uuid.New(),
	}
	fast := NewSound("v1", ptr(170.0), -8, 5, 0)
	slow := NewSound("v1", ptr(70.0), -20, 12, 0.1)
	f.cat = NewCatalog([]Track{
		{ID: f.a1, ArtistIDs: []uuid.UUID{f.artistA}, AlbumID: f.albumA1, GenreIDs: []uuid.UUID{f.rock}, Released: now.AddDate(-1, 0, 0), Plays: 10, Sound: fast},
		{ID: f.a2, ArtistIDs: []uuid.UUID{f.artistA}, AlbumID: f.albumA1, GenreIDs: []uuid.UUID{f.rock}, Released: now.AddDate(-1, 0, 0), Plays: 5, Sound: fast},
		{ID: f.a3, ArtistIDs: []uuid.UUID{f.artistA}, AlbumID: f.albumA2, GenreIDs: []uuid.UUID{f.rock}, Released: now.AddDate(0, 0, -7), Plays: 1},
		{ID: f.b1, ArtistIDs: []uuid.UUID{f.artistB}, AlbumID: f.albumB, GenreIDs: []uuid.UUID{f.jazz}, Released: now.AddDate(-2, 0, 0), Plays: 1000, Sound: slow},
		{ID: f.b2, ArtistIDs: []uuid.UUID{f.artistB}, AlbumID: f.albumB, GenreIDs: []uuid.UUID{f.jazz}, Released: now.AddDate(-2, 0, 0), Plays: 2, Sound: slow},
	}, now)
	return f
}

func ptr[T any](v T) *T { return &v }

func ids(s []Scored) []uuid.UUID {
	out := make([]uuid.UUID, len(s))
	for i, x := range s {
		out[i] = x.ID
	}
	return out
}

func TestRecommendFollowsTaste(t *testing.T) {
	f := newFixture()
	s := Signals{
		UserID:      uuid.New(),
		History:     map[uuid.UUID]Interaction{f.a1: {Plays: 3, Completions: 3}},
		LikedTracks: map[uuid.UUID]bool{f.a2: true},
	}
	tracks, artists := f.cat.Recommend(s, 10, 10)
	got := ids(tracks)
	if slices.Contains(got, f.a2) {
		t.Fatal("liked tracks must not be recommended")
	}
	// a3: same artist and genre, fresh — first, despite b1 being far more popular.
	if got[0] != f.a3 {
		t.Fatalf("ranking %v, want %v first", got, f.a3)
	}
	if !slices.Contains(tracks[0].Reasons, "artist") {
		t.Fatalf("reasons %v", tracks[0].Reasons)
	}
	// a1 was listened to the end: still there, but penalised below a3.
	if i := slices.Index(got, f.a1); i <= 0 {
		t.Fatalf("familiar track position %d in %v", i, got)
	}
	if artists[0].ID != f.artistA {
		t.Fatalf("artists %v", ids(artists))
	}
}

func TestSkipsExcludeAndPushAway(t *testing.T) {
	f := newFixture()
	s := Signals{UserID: uuid.New(), History: map[uuid.UUID]Interaction{
		f.b1: {Plays: 3, Skips: 3},
		f.a1: {Plays: 1, Completions: 1},
	}}
	tracks, _ := f.cat.Recommend(s, 10, 10)
	got := ids(tracks)
	if slices.Contains(got, f.b1) {
		t.Fatal("a mostly skipped track must be excluded")
	}
	if slices.Index(got, f.b2) < slices.Index(got, f.a2) {
		t.Fatalf("skipped artist ranked above the liked one: %v", got)
	}
}

func TestLikedAlbumCountsOnceAndExcludesItsTracks(t *testing.T) {
	f := newFixture()
	s := Signals{UserID: uuid.New(), LikedAlbums: map[uuid.UUID]bool{f.albumB: true}}
	p := f.cat.Profile(s)
	if p.artists[f.artistB] != signalAlbumLike || p.genres[f.jazz] != signalAlbumLike {
		t.Fatalf("album like: artists %v genres %v", p.artists, p.genres)
	}
	tracks, _ := f.cat.Recommend(s, 10, 10)
	if got := ids(tracks); slices.Contains(got, f.b1) || slices.Contains(got, f.b2) {
		t.Fatalf("tracks of a liked album recommended: %v", got)
	}
}

func TestSoundSimilarityBlendsIntoStyle(t *testing.T) {
	f := newFixture()
	p := f.cat.Profile(Signals{History: map[uuid.UUID]Interaction{f.a1: {Plays: 1, Completions: 1}}})
	if p.sound == nil {
		t.Fatal("profile without sound")
	}
	near := f.cat.components(p, f.cat.byID[f.a2]).style
	far := f.cat.components(p, f.cat.byID[f.b2]).style
	if near <= far || math.Abs(near-1) > 1e-9 {
		t.Fatalf("style near %v far %v", near, far)
	}
	// Different analyzer versions are not compared: style is genre only.
	other := *f.cat.byID[f.a2]
	other.Sound = NewSound("v2", nil, -8, 5, 0)
	if got := f.cat.components(p, &other).style; got != 1 {
		t.Fatalf("cross-version style %v", got)
	}
}

func TestFollowsRaiseArtistAffinity(t *testing.T) {
	f := newFixture()
	tracks, _ := f.cat.Recommend(Signals{FollowedArtists: map[uuid.UUID]bool{f.artistB: true}}, 2, 2)
	for _, tr := range tracks {
		if tr.ID != f.b1 && tr.ID != f.b2 {
			t.Fatalf("followed artist not on top: %v", ids(tracks))
		}
	}
}

func TestPopularAndEmptySignals(t *testing.T) {
	f := newFixture()
	if !(Signals{}).Empty() {
		t.Fatal("empty signals")
	}
	tracks, artists := f.cat.Popular(2, 1)
	if tracks[0].ID != f.b1 || len(tracks) != 2 || artists[0].ID != f.artistB {
		t.Fatalf("popular %v %v", ids(tracks), ids(artists))
	}
	if NewCatalog(nil, now).Len() != 0 {
		t.Fatal("empty catalog")
	}
}

func TestFreshnessHalvesEveryHalfLife(t *testing.T) {
	c := NewCatalog(nil, now)
	got := c.freshness(&Track{Released: now.Add(-freshnessHalfLife)})
	if math.Abs(got-0.5) > 1e-9 || c.freshness(&Track{}) != 0 || c.freshness(&Track{Released: now.Add(time.Hour)}) != 1 {
		t.Fatalf("freshness %v", got)
	}
}
