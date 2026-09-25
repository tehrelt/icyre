package catalog

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/library/internal/domain"
)

func TestExists(t *testing.T) {
	ready, deleted, album := uuid.New(), uuid.New(), uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/tracks/" + ready.String():
			_, _ = io.WriteString(w, `{"status":"READY"}`)
		case "/api/v1/tracks/" + deleted.String():
			_, _ = io.WriteString(w, `{"status":"DELETED"}`)
		case "/api/v1/albums/" + album.String():
			_, _ = io.WriteString(w, `{"id":"x"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	ctx := context.Background()
	if err := c.Exists(ctx, domain.KindTrack, ready); err != nil {
		t.Fatal(err)
	}
	if err := c.Exists(ctx, domain.KindAlbum, album); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{deleted, uuid.New()} {
		if err := c.Exists(ctx, domain.KindTrack, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatal(err)
		}
	}
}
