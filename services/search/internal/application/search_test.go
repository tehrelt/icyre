package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tehrelt/icyre/libs/contracts/search"
)

type fakeEngine struct {
	page       Page
	sizes      Sizes
	correction string
	corrected  int
}

func (f *fakeEngine) Search(_ context.Context, _ string, s Sizes) (Page, error) {
	f.sizes = s
	return f.page, nil
}
func (f *fakeEngine) Suggest(_ context.Context, q string, limit int) ([]Suggestion, error) {
	return []Suggestion{{Kind: "artist", Text: q, Score: float64(limit)}}, nil
}
func (f *fakeEngine) Correct(context.Context, string) (string, error) {
	f.corrected++
	return f.correction, nil
}

func TestValidation(t *testing.T) {
	svc := New(&fakeEngine{})
	for _, q := range []string{"", "   ", strings.Repeat("x", MaxQueryLength+1)} {
		if _, err := svc.Search(context.Background(), q, TypeAll, 10); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("%q: %v", q, err)
		}
	}
	if q, _ := Normalize("  prism \t hours "); q != "prism hours" {
		t.Fatal(q)
	}
	if _, ok := ParseType("songs"); ok {
		t.Fatal("unknown type accepted")
	}
}

func TestSizes(t *testing.T) {
	e := &fakeEngine{}
	svc := New(e)
	_, _ = svc.Search(context.Background(), "nova", TypeAll, 6)
	if e.sizes != (Sizes{Tracks: 4, Artists: 6, Albums: 6, Playlists: 6}) {
		t.Fatalf("all: %+v", e.sizes)
	}
	_, _ = svc.Search(context.Background(), "nova", TypeAlbums, 500)
	if e.sizes != (Sizes{Albums: MaxLimit}) {
		t.Fatalf("albums: %+v", e.sizes)
	}
}

func TestTopResult(t *testing.T) {
	artist := Hit[search.Artist]{Doc: search.Artist{Name: "Nova Hale"}, Score: 5}
	album := Hit[search.Album]{Doc: search.Album{Title: "Prism Hours"}, Score: 9}
	cases := []struct {
		q    string
		page Page
		want TopKind
	}{
		{"nova", Page{Artists: []Hit[search.Artist]{artist}, Albums: []Hit[search.Album]{album}}, TopArtist},
		{"prism", Page{Artists: []Hit[search.Artist]{artist}, Albums: []Hit[search.Album]{album}}, TopAlbum},
		{"hours", Page{Artists: []Hit[search.Artist]{artist}, Albums: []Hit[search.Album]{album}}, TopAlbum}, // by score
		{"x", Page{Playlists: []Hit[search.Playlist]{{Score: 1}}}, TopPlaylist},
		{"x", Page{Tracks: []Hit[search.Track]{{Score: 1}}}, ""},
	}
	for _, c := range cases {
		if got := topResult(c.q, c.page); got != c.want {
			t.Errorf("%s: %q, want %q", c.q, got, c.want)
		}
	}
}

func TestDidYouMeanOnlyWithoutResults(t *testing.T) {
	e := &fakeEngine{correction: "nova hale"}
	svc := New(e)
	res, _ := svc.Search(context.Background(), "nvoa hale", TypeAll, 6)
	if res.DidYouMean != "nova hale" {
		t.Fatal(res.DidYouMean)
	}
	e.page = Page{Counts: Counts{Tracks: 1}}
	res, _ = svc.Search(context.Background(), "nvoa", TypeAll, 6)
	if res.DidYouMean != "" || e.corrected != 1 {
		t.Fatalf("correction with results: %q (%d calls)", res.DidYouMean, e.corrected)
	}
}
