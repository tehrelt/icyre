package history

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

func TestRecentSources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("limit") != "6" {
			t.Errorf("limit %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[{"source":"album:a","playedAt":"2026-09-25T10:00:00Z"},{"source":"playlist:p"}]}`)
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	got, err := c.RecentSources(ports.WithUserToken(context.Background(), "good"), 6)
	if err != nil || len(got) != 2 || got[0] != "album:a" {
		t.Fatal(got, err)
	}
	for _, ctx := range []context.Context{context.Background(), ports.WithUserToken(context.Background(), "expired")} {
		if got, err := c.RecentSources(ctx, 6); err != nil || len(got) != 0 {
			t.Fatalf("no history expected: %v %v", got, err)
		}
	}
}
