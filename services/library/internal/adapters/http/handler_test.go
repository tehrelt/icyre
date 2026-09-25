package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/library/internal/application"
	"github.com/tehrelt/icyre/services/library/internal/domain"
)

type fakeLib struct {
	saved map[uuid.UUID]time.Time
	down  bool
}

func (f *fakeLib) Save(_ context.Context, _ uuid.UUID, _ domain.Kind, id uuid.UUID) (domain.Item, error) {
	if id == uuid.Nil {
		return domain.Item{}, domain.ErrNotFound
	}
	f.saved[id] = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	return domain.Item{EntityID: id}, nil
}
func (f *fakeLib) Remove(_ context.Context, _ uuid.UUID, _ domain.Kind, id uuid.UUID) error {
	delete(f.saved, id)
	return nil
}
func (f *fakeLib) List(context.Context, uuid.UUID, domain.Kind, *domain.Cursor, int) (application.Page, error) {
	if f.down {
		return application.Page{}, errors.New("db down")
	}
	var p application.Page
	for id, at := range f.saved {
		p.Items = append(p.Items, domain.Item{EntityID: id, SavedAt: at})
	}
	p.Next = &domain.Cursor{SavedAt: time.Unix(1, 0), EntityID: uuid.New()}
	return p, nil
}
func (f *fakeLib) Contains(_ context.Context, _ uuid.UUID, _ domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, id := range ids {
		if _, ok := f.saved[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
func (f *fakeLib) Counts(context.Context, uuid.UUID) (domain.Counts, error) {
	return domain.Counts{Tracks: len(f.saved), Albums: 2}, nil
}

func setup(t *testing.T) (http.Handler, *fakeLib, string) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	lib := &fakeLib{saved: map[uuid.UUID]time.Time{}}
	mux := http.NewServeMux()
	NewHandler(lib, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s1", RegisteredClaims: jwt.RegisteredClaims{
		Subject: uuid.NewString(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return mux, lib, s
}

func do(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLibraryAPI(t *testing.T) {
	h, lib, tok := setup(t)
	id := uuid.New()
	if rec := do(h, http.MethodPut, "/api/v1/me/library/tracks/"+id.String(), ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	for range 2 {
		if rec := do(h, http.MethodPut, "/api/v1/me/library/tracks/"+id.String(), tok); rec.Code != http.StatusNoContent {
			t.Fatalf("put: %d %s", rec.Code, rec.Body)
		}
	}
	if rec := do(h, http.MethodPut, "/api/v1/me/library/albums/"+uuid.Nil.String(), tok); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "ALBUM_NOT_FOUND") {
		t.Fatalf("unknown album: %d %s", rec.Code, rec.Body)
	}
	rec := do(h, http.MethodGet, "/api/v1/me/library/tracks?limit=10", tok)
	var list listResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Data) != 1 || list.Data[0].TrackID != id.String() || !list.Pagination.HasMore || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodGet, "/api/v1/me/library/tracks/contains?ids="+id.String()+","+uuid.NewString(), tok)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"data":["`+id.String()+`"]}`+"\n" {
		t.Fatalf("contains: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/api/v1/me/library/summary", tok); rec.Body.String() != `{"albums":2,"tracks":1}`+"\n" {
		t.Fatalf("summary: %s", rec.Body)
	}
	for range 2 {
		if rec := do(h, http.MethodDelete, "/api/v1/me/library/tracks/"+id.String(), tok); rec.Code != http.StatusNoContent {
			t.Fatalf("delete: %d", rec.Code)
		}
	}
	for path, want := range map[string]int{
		"/api/v1/me/library/tracks?limit=0":         http.StatusUnprocessableEntity,
		"/api/v1/me/library/tracks?cursor=abc":      http.StatusUnprocessableEntity,
		"/api/v1/me/library/tracks/contains?ids=":   http.StatusUnprocessableEntity,
		"/api/v1/me/library/tracks/contains?ids=zz": http.StatusUnprocessableEntity,
	} {
		if rec := do(h, http.MethodGet, path, tok); rec.Code != want {
			t.Errorf("%s: %d want %d", path, rec.Code, want)
		}
	}
	lib.down = true
	if rec := do(h, http.MethodGet, "/api/v1/me/library/tracks", tok); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("db down: %d", rec.Code)
	}
}
