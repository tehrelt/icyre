package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/libs/contracts/search"
	"github.com/tehrelt/icyre/services/search/internal/application"
)

type fakeEngine struct{ fail bool }

func (f fakeEngine) Search(context.Context, string, application.Sizes) (application.Page, error) {
	if f.fail {
		return application.Page{}, errors.New("opensearch down")
	}
	return application.Page{
		Artists: []application.Hit[search.Artist]{{Doc: search.Artist{ID: "ar1", Name: "Nova Hale", Verified: true}, Score: 3}},
		Albums:  []application.Hit[search.Album]{{Doc: search.Album{ID: "al1", Title: "Prism Hours", ArtistNames: []string{"Nova Hale"}, ReleaseDate: "2026-03-06", AlbumType: "ALBUM"}, Score: 2}},
		Tracks:  []application.Hit[search.Track]{{Doc: search.Track{ID: "t1", Title: "Glass Tides", ArtistNames: []string{"Nova Hale", "Kai Frost"}, AlbumID: "al1", AlbumTitle: "Prism Hours", DurationMs: 227400, Available: true}}},
		Counts:  application.Counts{Tracks: 1, Artists: 1, Albums: 1},
	}, nil
}
func (fakeEngine) Suggest(context.Context, string, int) ([]application.Suggestion, error) {
	return []application.Suggestion{{Kind: "artist", ID: "ar1", Text: "Nova Hale", Subtitle: "Artist"}}, nil
}
func (fakeEngine) Correct(context.Context, string) (string, error) { return "", nil }

func get(h http.Handler, url string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func setup(fail bool) http.Handler {
	mux := http.NewServeMux()
	NewHandler(application.New(fakeEngine{fail: fail}), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	return mux
}

func TestSearchResponseShape(t *testing.T) {
	rec := get(setup(false), "/api/v1/search?q=nova&type=all&limit=6")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != cacheControl {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	top := body["topResult"].(map[string]any)
	item := top["item"].(map[string]any)
	if item["kind"] != "artist" || item["name"] != "Nova Hale" || top["verified"] != true {
		t.Fatalf("topResult %v", top)
	}
	tr := body["tracks"].([]any)[0].(map[string]any)
	if tr["artistName"] != "Nova Hale, Kai Frost" || tr["durationSec"].(float64) != 227 || tr["coverUrl"] != nil || tr["available"] != true {
		t.Fatalf("track %v", tr)
	}
	al := body["albums"].([]any)[0].(map[string]any)
	if al["year"].(float64) != 2026 || al["kind"] != "album" || al["albumType"] != "ALBUM" {
		t.Fatalf("album %v", al)
	}
	if body["didYouMean"] != nil || body["playlists"] == nil {
		t.Fatalf("didYouMean/playlists %v %v", body["didYouMean"], body["playlists"])
	}
}

func TestSearchErrors(t *testing.T) {
	h := setup(false)
	for url, want := range map[string]int{
		"/api/v1/search?q=":                  http.StatusUnprocessableEntity,
		"/api/v1/search?q=a&type=songs":      http.StatusUnprocessableEntity,
		"/api/v1/search?q=a&limit=x":         http.StatusUnprocessableEntity,
		"/api/v1/search/suggest?q=no":        http.StatusOK,
		"/api/v1/search/suggest?q=%20%20%20": http.StatusUnprocessableEntity,
	} {
		if rec := get(h, url); rec.Code != want {
			t.Errorf("%s: %d, want %d (%s)", url, rec.Code, want, rec.Body)
		}
	}
	if rec := get(setup(true), "/api/v1/search?q=nova"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("engine down: %d", rec.Code)
	}
}
