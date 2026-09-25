//go:build integration

package opensearch

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/search"
	"github.com/tehrelt/icyre/libs/platform/opensearch"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/indices"
)

// newIndex creates a throwaway tracks index with the production template.
func newIndex(t *testing.T) (*opensearch.Client, string) {
	t.Helper()
	url := os.Getenv("OPENSEARCH_URL")
	if url == "" {
		t.Skip("OPENSEARCH_URL is not set")
	}
	c := opensearch.New(opensearch.Config{URL: url}, nil)
	ctx := context.Background()
	name := fmt.Sprintf("it-%d-tracks-v1", time.Now().UnixNano())
	tpl := indices.Template(search.AliasTracks, indices.Options{})["template"]
	if err := c.CreateIndex(ctx, name, tpl); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.DeleteIndex(context.Background(), name) })
	return c, name
}

func hits(t *testing.T, c *opensearch.Client, index string, query map[string]any) []string {
	t.Helper()
	var res struct {
		Hits struct {
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := c.Do(context.Background(), http.MethodPost, "/"+index+"/_search", map[string]any{"query": query}, &res); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(res.Hits.Hits))
	for i, h := range res.Hits.Hits {
		ids[i] = h.ID
	}
	return ids
}

func TestIndexVersioningAndAnalyzers(t *testing.T) {
	c, name := newIndex(t)
	ctx := context.Background()
	x := New(c, true)
	at := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	doc := func(title string, ts time.Time) search.Track {
		return search.Track{ID: "t1", Title: title, ArtistNames: []string{"Nova Hale"}, AlbumTitle: "Prism Hours", Available: true, UpdatedAt: ts}
	}

	if err := x.Apply(ctx, []application.Write{
		{Index: name, ID: "t1", Version: at.UnixMilli(), Doc: doc("Café Tides", at)},
		{Index: name, ID: "t2", Version: at.UnixMilli(), Doc: search.Track{ID: "t2", Title: "Rime", UpdatedAt: at}},
	}); err != nil {
		t.Fatal(err)
	}
	// A stale event (older version) is accepted and ignored.
	if err := x.Apply(ctx, []application.Write{{Index: name, ID: "t1", Version: at.Add(-time.Hour).UnixMilli(), Doc: doc("Old Title", at)}}); err != nil {
		t.Fatalf("stale write must not fail: %v", err)
	}
	if got := hits(t, c, name, map[string]any{"match": map[string]any{"title": "old"}}); len(got) != 0 {
		t.Fatalf("stale write overwrote the document: %v", got)
	}
	// Accent folding and autocomplete.
	if got := hits(t, c, name, map[string]any{"match": map[string]any{"title": "cafe"}}); len(got) != 1 {
		t.Fatalf("asciifolding: %v", got)
	}
	if got := hits(t, c, name, map[string]any{"match": map[string]any{"title.autocomplete": "ti"}}); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("autocomplete: %v", got)
	}
	if got := hits(t, c, name, map[string]any{"match": map[string]any{"albumTitle.autocomplete": "pri"}}); len(got) != 1 {
		t.Fatalf("autocomplete on album title: %v", got)
	}
	// The keyword normalizer applies to queries too: exact matches ignore case and accents.
	if got := hits(t, c, name, map[string]any{"term": map[string]any{"title.keyword": "CAFE TIDES"}}); len(got) != 1 {
		t.Fatalf("keyword normalizer: %v", got)
	}
	if got := hits(t, c, name, map[string]any{"prefix": map[string]any{"title.keyword": "cafe t"}}); len(got) != 1 {
		t.Fatalf("keyword prefix: %v", got)
	}

	// Deletes, including of documents that do not exist.
	if err := x.Apply(ctx, []application.Write{
		{Index: name, ID: "t2", Version: at.Add(time.Minute).UnixMilli(), Delete: true},
		{Index: name, ID: "ghost", Delete: true},
	}); err != nil {
		t.Fatal(err)
	}
	if got := hits(t, c, name, map[string]any{"match_all": map[string]any{}}); len(got) != 1 {
		t.Fatalf("after delete: %v", got)
	}

	// The strict mapping rejects fields outside the contract.
	if err := x.Apply(ctx, []application.Write{{Index: name, ID: "t3", Doc: map[string]any{"id": "t3", "mood": "calm"}}}); err == nil {
		t.Fatal("unknown field accepted")
	}
}
