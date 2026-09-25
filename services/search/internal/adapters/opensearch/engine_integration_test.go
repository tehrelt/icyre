//go:build integration

package opensearch

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/search"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/services/search/internal/application"
)

// seed builds isolated indices with the production template and a small
// catalogue shaped like the product canvas.
func seed(t *testing.T) *Engine {
	t.Helper()
	url := os.Getenv("OPENSEARCH_URL")
	if url == "" {
		t.Skip("OPENSEARCH_URL is not set")
	}
	c := opensearch.New(opensearch.Config{URL: url}, nil)
	ctx := context.Background()
	prefix := fmt.Sprintf("it%d-", time.Now().UnixNano())
	for _, alias := range search.Aliases {
		name := prefix + alias
		if err := c.CreateIndex(ctx, name, search.Template(alias, 1, 0)["template"]); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.DeleteIndex(context.Background(), name) })
	}
	now := time.Now().UTC()
	old := now.AddDate(-3, 0, 0)
	var ops []opensearch.BulkOp
	add := func(alias, id string, doc any) {
		ops = append(ops, opensearch.BulkOp{Action: opensearch.ActionIndex, Index: prefix + alias, ID: id, Doc: doc})
	}
	add(search.AliasArtists, "ar-nova", search.Artist{ID: "ar-nova", Name: "Nova Hale", Popularity: 100, UpdatedAt: now})
	add(search.AliasArtists, "ar-novaline", search.Artist{ID: "ar-novaline", Name: "Novaline", Popularity: 5, UpdatedAt: now})
	add(search.AliasArtists, "ar-terra", search.Artist{ID: "ar-terra", Name: "Terra Nova", UpdatedAt: now})
	add(search.AliasArtists, "ar-mira", search.Artist{ID: "ar-mira", Name: "Mira Solen", UpdatedAt: now})
	add(search.AliasAlbums, "al-prism", search.Album{ID: "al-prism", Title: "Prism Hours", ArtistNames: []string{"Nova Hale"}, ReleaseDate: now.Format("2006-01-02"), UpdatedAt: now})
	add(search.AliasAlbums, "al-prism-old", search.Album{ID: "al-prism-old", Title: "Prism", ArtistNames: []string{"Terra Nova"}, ReleaseDate: old.Format("2006-01-02"), UpdatedAt: old})
	add(search.AliasAlbums, "al-winter", search.Album{ID: "al-winter", Title: "Winter Index", ArtistNames: []string{"Nova Hale"}, ReleaseDate: "2024-11-15", UpdatedAt: now})
	add(search.AliasAlbums, "al-echo-old", search.Album{ID: "al-echo-old", Title: "Echoes", ReleaseDate: old.Format("2006-01-02"), UpdatedAt: old})
	add(search.AliasAlbums, "al-echo-new", search.Album{ID: "al-echo-new", Title: "Echoes", ReleaseDate: now.Format("2006-01-02"), UpdatedAt: now})
	add(search.AliasTracks, "t-glass", search.Track{ID: "t-glass", Title: "Glass Tides", ArtistNames: []string{"Nova Hale"}, AlbumTitle: "Prism Hours", Available: true, UpdatedAt: now})
	add(search.AliasTracks, "t-cafe", search.Track{ID: "t-cafe", Title: "Café Lights", ArtistNames: []string{"Mira Solen"}, AlbumTitle: "Salt Lanterns", Available: true, UpdatedAt: now})
	items, err := c.Bulk(ctx, ops, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Status >= 300 {
			t.Fatalf("seed %s: %v", it.ID, it.Error)
		}
	}
	return NewWithPrefix(c, prefix)
}

func ids[T any](hits []application.Hit[T], id func(T) string) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = id(h.Doc)
	}
	return out
}

var (
	artistID = func(a search.Artist) string { return a.ID }
	albumID  = func(a search.Album) string { return a.ID }
	trackID  = func(t search.Track) string { return t.ID }
	all      = application.Sizes{Tracks: 10, Artists: 10, Albums: 10, Playlists: 10}
)

func TestRanking(t *testing.T) {
	e := seed(t)
	ctx := context.Background()

	p, err := e.Search(ctx, "nova", all)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(p.Artists, artistID)
	if len(got) < 2 || got[0] != "ar-nova" {
		t.Fatalf("nova artists: %v", got)
	}
	// Nova Hale's albums match through the artist name, and a track through artist too.
	if p.Counts.Albums < 2 || p.Counts.Tracks != 1 {
		t.Fatalf("nova counts: %+v", p.Counts)
	}

	// Search as you type.
	if p, _ = e.Search(ctx, "pri", all); len(p.Albums) == 0 || p.Albums[0].Doc.Title[:5] != "Prism" {
		t.Fatalf("prefix: %v", ids(p.Albums, albumID))
	}
	// Exact title wins, and between equal matches the recent release wins.
	if p, _ = e.Search(ctx, "prism hours", all); ids(p.Albums, albumID)[0] != "al-prism" {
		t.Fatalf("exact: %v", ids(p.Albums, albumID))
	}
	if p, _ = e.Search(ctx, "prism", all); ids(p.Albums, albumID)[0] != "al-prism-old" {
		// "Prism" is an exact title match: exactness beats recency.
		t.Fatalf("exact beats recency: %v", ids(p.Albums, albumID))
	}
	// Same text match: the recent release ranks first.
	if p, _ = e.Search(ctx, "echoes", all); ids(p.Albums, albumID)[0] != "al-echo-new" {
		t.Fatalf("recency: %v", ids(p.Albums, albumID))
	}
	// Typos and accents.
	if p, _ = e.Search(ctx, "prsim", all); len(p.Albums) == 0 {
		t.Fatal("fuzzy match failed")
	}
	if p, _ = e.Search(ctx, "cafe", all); len(p.Tracks) != 1 || ids(p.Tracks, trackID)[0] != "t-cafe" {
		t.Fatalf("accent folding: %v", ids(p.Tracks, trackID))
	}
	// Sizes only limit hits, never counts.
	p, _ = e.Search(ctx, "nova", application.Sizes{Albums: 1})
	if len(p.Artists) != 0 || len(p.Albums) != 1 || p.Counts.Artists < 2 {
		t.Fatalf("sizes: %+v", p.Counts)
	}
	if p, _ = e.Search(ctx, "zzzz", all); p.Counts != (application.Counts{}) {
		t.Fatalf("no match: %+v", p.Counts)
	}
}

func TestSuggestAndCorrect(t *testing.T) {
	e := seed(t)
	ctx := context.Background()
	s, err := e.Suggest(ctx, "no", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) == 0 || s[0].Kind != "artist" || s[0].Text != "Nova Hale" {
		t.Fatalf("suggest: %+v", s)
	}
	if len(s) > 5 {
		t.Fatal("limit ignored")
	}

	fix, err := e.Correct(ctx, "nvao hale")
	if err != nil {
		t.Fatal(err)
	}
	if fix != "nova hale" {
		t.Fatalf("correction: %q", fix)
	}
	if fix, _ := e.Correct(ctx, "nova hale"); fix != "" {
		t.Fatalf("correct query corrected: %q", fix)
	}
}
