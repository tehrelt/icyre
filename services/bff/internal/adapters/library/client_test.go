package library

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

func TestSavedTracks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer good":
			if r.URL.Query().Get("ids") != "t1,t2" {
				t.Errorf("ids %q", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"data":["t2"]}`)
		case "Bearer expired":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	ids := []string{"t1", "t2"}

	got, err := c.SavedTracks(ports.WithUserToken(context.Background(), "good"), ids)
	if err != nil || !got["t2"] || got["t1"] {
		t.Fatal(got, err)
	}
	if got, err := c.SavedTracks(context.Background(), ids); err != nil || len(got) != 0 {
		t.Fatalf("anonymous: %v %v", got, err)
	}
	if got, err := c.SavedTracks(ports.WithUserToken(context.Background(), "expired"), ids); err != nil || len(got) != 0 {
		t.Fatalf("expired: %v %v", got, err)
	}
	if _, err := c.SavedTracks(ports.WithUserToken(context.Background(), "broken"), ids); err == nil {
		t.Fatal("upstream failure hidden")
	}
}
