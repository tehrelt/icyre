package playlist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

func TestClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/playlists/p1":
			_, _ = w.Write([]byte(`{"id":"p1","ownerId":"u1","title":"Mix","updatedAt":"2026-09-26T10:00:00Z","tracks":[{"trackId":"t2","position":1},{"trackId":"t1","position":3}]}`))
		case "/api/v1/users/u1":
			_, _ = w.Write([]byte(`{"id":"u1","username":"mira","displayName":""}`))
		case "/api/v1/users/down":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	c := New(srv.URL+"/", srv.Client())
	p, err := c.GetPlaylist(ctx, "p1")
	if err != nil || p.OwnerID != "u1" || len(p.TrackIDs) != 2 || p.TrackIDs[0] != "t2" || p.UpdatedAt.Year() != 2026 {
		t.Fatal(p, err)
	}
	if _, err := c.GetPlaylist(ctx, "gone"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal(err)
	}

	u := NewProfiles(srv.URL, srv.Client())
	if name, err := u.DisplayName(ctx, "u1"); err != nil || name != "mira" {
		t.Fatal(name, err)
	}
	if _, err := u.DisplayName(ctx, "ghost"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := u.DisplayName(ctx, "down"); err == nil || errors.Is(err, ports.ErrNotFound) {
		t.Fatal("5xx must be an upstream error", err)
	}
}
