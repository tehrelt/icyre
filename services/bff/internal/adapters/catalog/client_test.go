package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

func TestClientMapsCatalogContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/albums/a1":
			_, _ = w.Write([]byte(`{"id":"a1","title":"Prism Hours","albumType":"ALBUM","releaseDate":"2026-03-06","artistIds":["n1"],"genreIds":[],"createdAt":"2026-01-01T00:00:00Z"}`))
		case "/api/v1/albums/a1/tracks":
			_, _ = w.Write([]byte(`{"data":[{"id":"t1","albumId":"a1","artistIds":["n1"],"title":"Glass Tides","durationMs":227000,"trackNumber":2,"discNumber":1,"explicit":false,"isrc":null,"status":"READY"}]}`))
		case "/api/v1/artists":
			if r.URL.Query().Get("ids") != "n1,n2" {
				t.Errorf("ids = %q", r.URL.Query().Get("ids"))
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"n1","name":"Nova Hale"}]}`))
		case "/api/v1/albums":
			if r.URL.Query().Get("limit") != "6" {
				t.Errorf("limit = %q", r.URL.Query().Get("limit"))
			}
			_, _ = w.Write([]byte(`{"data":[],"pagination":{"nextCursor":null,"hasMore":false}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"ALBUM_NOT_FOUND","message":"Album not found"}}`))
		}
	}))
	defer srv.Close()

	c := New(srv.URL, srv.Client(), prometheus.NewRegistry())
	ctx := context.Background()

	a, err := c.GetAlbum(ctx, "a1")
	if err != nil || a.Title != "Prism Hours" || a.ReleaseDate.Year() != 2026 || a.ArtistIDs[0] != "n1" {
		t.Fatalf("album = %+v, err = %v", a, err)
	}
	tracks, err := c.AlbumTracks(ctx, "a1")
	if err != nil || len(tracks) != 1 || tracks[0].Duration != 227*time.Second || tracks[0].Status != "READY" {
		t.Fatalf("tracks = %+v, err = %v", tracks, err)
	}
	artists, err := c.Artists(ctx, []string{"n1", "n2"})
	if err != nil || len(artists) != 1 || artists[0].Name != "Nova Hale" {
		t.Fatalf("artists = %+v, err = %v", artists, err)
	}
	if latest, err := c.LatestAlbums(ctx, 6); err != nil || len(latest) != 0 {
		t.Fatalf("latest = %+v, err = %v", latest, err)
	}
	if _, err := c.GetAlbum(ctx, "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClientSurfacesUpstreamErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := New(srv.URL, srv.Client(), nil).Genres(context.Background())
	if err == nil || errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}
