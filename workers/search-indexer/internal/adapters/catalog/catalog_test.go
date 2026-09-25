package catalog

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path + "?" + r.URL.RawQuery {
		case "/api/v1/albums?limit=100":
			_, _ = io.WriteString(w, `{"data":[{"id":"al1","title":"A"}],"pagination":{"nextCursor":"c2","hasMore":true}}`)
		case "/api/v1/albums?limit=100&cursor=c2":
			_, _ = io.WriteString(w, `{"data":[{"id":"al2","title":"B","artistIds":["ar1"]}],"pagination":{"hasMore":false}}`)
		case "/api/v1/albums/al2/tracks?":
			_, _ = io.WriteString(w, `{"data":[{"id":"t1","title":"T","status":"READY","durationMs":1000}]}`)
		case "/api/v1/artists?ids=ar1%2Car2":
			_, _ = io.WriteString(w, `{"data":[{"id":"ar1","name":"Nova Hale"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	ctx := context.Background()

	var seen []string
	if err := c.EachAlbum(ctx, func(a application.Album) error { seen = append(seen, a.ID); return nil }); err != nil || len(seen) != 2 {
		t.Fatal(seen, err)
	}
	tracks, err := c.AlbumTracks(ctx, "al2")
	if err != nil || len(tracks) != 1 || tracks[0].Status != "READY" {
		t.Fatal(tracks, err)
	}
	artists, err := c.Artists(ctx, []string{"ar2", "ar1", "ar1"})
	if err != nil || artists["ar1"].Name != "Nova Hale" || len(artists) != 1 {
		t.Fatal(artists, err)
	}
	if _, err := c.Album(ctx, "nope"); !errors.Is(err, application.ErrNotFound) {
		t.Fatal(err)
	}
}
