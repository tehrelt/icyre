package directory

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/social/internal/domain"
)

func TestExists(t *testing.T) {
	known := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/users/" + known.String(), "/api/v1/artists/" + known.String():
			_, _ = w.Write([]byte(`{}`))
		case "/api/v1/artists/" + uuid.Nil.String():
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, srv.URL+"/", srv.Client())
	ctx := context.Background()
	for _, typ := range []domain.TargetType{domain.TargetUser, domain.TargetArtist} {
		if err := c.Exists(ctx, domain.Target{Type: typ, ID: known}); err != nil {
			t.Errorf("%s known: %v", typ, err)
		}
		if err := c.Exists(ctx, domain.Target{Type: typ, ID: uuid.New()}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s unknown: %v", typ, err)
		}
	}
	if err := c.Exists(ctx, domain.Target{Type: domain.TargetArtist, ID: uuid.Nil}); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Errorf("upstream failure: %v", err)
	}
}
