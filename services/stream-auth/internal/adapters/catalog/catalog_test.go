package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

func TestStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/tracks/t1":
			_, _ = w.Write([]byte(`{"id":"t1","status":"READY","title":"x"}`))
		case "/api/v1/tracks/boom":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	ctx := context.Background()
	if s, err := c.Status(ctx, "t1"); err != nil || s != "READY" {
		t.Fatal(s, err)
	}
	if _, err := c.Status(ctx, "nope"); !errors.Is(err, domain.ErrTrackNotFound) {
		t.Fatal(err)
	}
	if _, err := c.Status(ctx, "boom"); err == nil || errors.Is(err, domain.ErrTrackNotFound) {
		t.Fatal(err)
	}
}
