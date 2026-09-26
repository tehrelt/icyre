package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tehrelt/icyre/services/bff/internal/application"
	"github.com/tehrelt/icyre/services/bff/internal/ports"
	"github.com/tehrelt/icyre/services/bff/internal/views"
)

type stubPages struct{ err error }

func (s stubPages) Home(context.Context) (views.HomePage, error) {
	return views.HomePage{RecentlyPlayed: []any{}, NewReleases: views.Shelf[views.AlbumCard]{Items: []views.AlbumCard{}}}, s.err
}
func (s stubPages) Album(context.Context, string) (views.AlbumPage, error) {
	return views.AlbumPage{Album: views.AlbumHeader{Title: "Prism Hours", Tags: []string{}}}, s.err
}

func (s stubPages) Playlist(context.Context, string) (views.PlaylistPage, error) {
	return views.PlaylistPage{Playlist: views.PlaylistHeader{Title: "Late night"}, Tracks: []views.Track{}}, s.err
}

func serve(p Pages, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewHandler(p, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestPagesStatusMapping(t *testing.T) {
	cases := []struct {
		err    error
		path   string
		status int
		want   string
	}{
		{nil, "/api/v1/pages/album/x", 404, ""},
		{nil, "/api/v1/pages/albums/x", 200, `"title":"Prism Hours"`},
		{nil, "/api/v1/pages/home", 200, `"recentlyPlayed":[]`},
		{fmt.Errorf("album: %w", application.ErrNotFound), "/api/v1/pages/albums/x", 404, `"ALBUM_NOT_FOUND"`},
		{fmt.Errorf("tracks: %w", application.ErrUpstream), "/api/v1/pages/albums/x", 503, `"SERVICE_UNAVAILABLE"`},
		{errors.New("bug"), "/api/v1/pages/home", 500, `"INTERNAL"`},
		{nil, "/api/v1/pages/playlists/x", 200, `"title":"Late night"`},
		{fmt.Errorf("playlist: %w", application.ErrNotFound), "/api/v1/pages/playlists/x", 404, `"PLAYLIST_NOT_FOUND"`},
	}
	for _, c := range cases {
		rec := serve(stubPages{err: c.err}, c.path)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s (%v): status %d body %s", c.path, c.err, rec.Code, rec.Body)
		}
	}
}

// userSeen records the token the application layer received.
type userSeen struct {
	stubPages
	token *string
}

func (u userSeen) Album(ctx context.Context, id string) (views.AlbumPage, error) {
	*u.token = ports.UserToken(ctx)
	return u.stubPages.Album(ctx, id)
}

func TestUserContextAndCaching(t *testing.T) {
	var token string
	mux := http.NewServeMux()
	NewHandler(userSeen{token: &token}, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pages/albums/x", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if token != "abc" || rec.Header().Get("Cache-Control") != "private, no-cache" || rec.Header().Get("Vary") != "Authorization" {
		t.Fatalf("signed in: token %q, cache %q, vary %q", token, rec.Header().Get("Cache-Control"), rec.Header().Get("Vary"))
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/pages/albums/x", nil))
	if token != "" || rec.Header().Get("Cache-Control") != "private, max-age=30" {
		t.Fatalf("anonymous: token %q, cache %q", token, rec.Header().Get("Cache-Control"))
	}
}
