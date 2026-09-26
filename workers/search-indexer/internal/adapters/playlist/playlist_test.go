package playlist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

func TestClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path + "?" + r.URL.RawQuery {
		case "/api/v1/playlists/p1?":
			_, _ = w.Write([]byte(`{"id":"p1","ownerId":"u1","title":"Mix","trackCount":3,"updatedAt":"2026-09-26T10:00:00Z"}`))
		case "/internal/v1/playlists?limit=500&after=":
			_, _ = w.Write([]byte(`{"data":[{"id":"p1"},{"id":"p2"}],"nextAfter":"p2"}`))
		case "/internal/v1/playlists?limit=500&after=p2":
			_, _ = w.Write([]byte(`{"data":[],"nextAfter":""}`))
		case "/api/v1/users/u1?":
			_, _ = w.Write([]byte(`{"id":"u1","username":"nova","displayName":"Nova"}`))
		case "/api/v1/users/u2?":
			_, _ = w.Write([]byte(`{"id":"u2","username":"lune","displayName":""}`))
		case "/api/v1/users/down?":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	c := New(srv.URL+"/", srv.Client())

	p, err := c.Playlist(ctx, "p1")
	if err != nil || p.Title != "Mix" || p.TrackCount != 3 || p.OwnerID != "u1" {
		t.Fatal(p, err)
	}
	if _, err := c.Playlist(ctx, "gone"); !errors.Is(err, application.ErrNotFound) {
		t.Fatal(err)
	}
	var ids []string
	if err := c.EachPlaylist(ctx, func(p application.Playlist) error { ids = append(ids, p.ID); return nil }); err != nil || len(ids) != 2 {
		t.Fatal(ids, err)
	}

	u := NewProfiles(srv.URL, srv.Client())
	names, err := u.DisplayNames(ctx, []string{"u1", "u2", "ghost"})
	if err != nil || names["u1"] != "Nova" || names["u2"] != "lune" || len(names) != 2 {
		t.Fatal(names, err)
	}
	if _, err := u.DisplayNames(ctx, []string{"down"}); err == nil {
		t.Fatal("5xx must fail")
	}
}
