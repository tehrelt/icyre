package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

// fakeApp owns one playlist, id, with the tracks in order.
type fakeApp struct {
	id, owner uuid.UUID
	order     []uuid.UUID
	title     string
	deleted   bool
}

func (f *fakeApp) check(caller, id uuid.UUID) error {
	if id != f.id || f.deleted {
		return domain.ErrNotFound
	}
	if caller != f.owner {
		return domain.ErrForbidden
	}
	return nil
}
func (f *fakeApp) Create(context.Context, uuid.UUID, string) (domain.Playlist, error) {
	return domain.Playlist{}, nil
}
func (f *fakeApp) Get(context.Context, uuid.UUID) (domain.Playlist, []domain.Track, error) {
	return domain.Playlist{}, nil, nil
}
func (f *fakeApp) Mine(context.Context, uuid.UUID) ([]domain.Playlist, error) { return nil, nil }
func (f *fakeApp) AddTrack(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *fakeApp) RemoveTrack(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *fakeApp) Rename(_ context.Context, caller, id uuid.UUID, title string) (domain.Playlist, error) {
	if err := f.check(caller, id); err != nil {
		return domain.Playlist{}, err
	}
	t, err := domain.NormalizeTitle(title)
	if err != nil {
		return domain.Playlist{}, err
	}
	f.title = t
	return domain.Playlist{ID: id, OwnerID: caller, Title: t}, nil
}
func (f *fakeApp) Delete(_ context.Context, caller, id uuid.UUID) error {
	if err := f.check(caller, id); err != nil {
		return err
	}
	f.deleted = true
	return nil
}
func (f *fakeApp) Reorder(_ context.Context, caller, id uuid.UUID, order []uuid.UUID) error {
	if err := f.check(caller, id); err != nil {
		return err
	}
	current := make([]domain.Track, len(f.order))
	for i, t := range f.order {
		current[i] = domain.Track{TrackID: t}
	}
	if err := domain.CheckOrder(current, order); err != nil {
		return err
	}
	f.order = order
	return nil
}

func token(t *testing.T, priv ed25519.PrivateKey, sub uuid.UUID) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s1", RegisteredClaims: jwt.RegisteredClaims{
		Subject: sub.String(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEditRoutes(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	a, b := uuid.New(), uuid.New()
	app := &fakeApp{id: uuid.New(), owner: uuid.New(), order: []uuid.UUID{a, b}}
	mux := http.NewServeMux()
	NewHandler(app, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	owner, stranger := token(t, priv, app.owner), token(t, priv, uuid.New())
	base := "/api/v1/playlists/" + app.id.String()

	for _, tc := range []struct {
		name, method, path, tok, body string
		want                          int
	}{
		{"rename unauthenticated", http.MethodPatch, base, "", `{"title":"x"}`, 401},
		{"rename", http.MethodPatch, base, owner, `{"title":" Road  trip "}`, 200},
		{"rename blank", http.MethodPatch, base, owner, `{"title":" "}`, 422},
		{"rename stranger", http.MethodPatch, base, stranger, `{"title":"x"}`, 403},
		{"reorder", http.MethodPatch, base + "/tracks/order", owner, `{"trackIds":["` + b.String() + `","` + a.String() + `"]}`, 204},
		{"reorder partial", http.MethodPatch, base + "/tracks/order", owner, `{"trackIds":["` + b.String() + `"]}`, 409},
		{"reorder not uuid", http.MethodPatch, base + "/tracks/order", owner, `{"trackIds":["nope"]}`, 422},
		{"reorder missing list", http.MethodPatch, base + "/tracks/order", owner, `{}`, 422},
		{"delete stranger", http.MethodDelete, base, stranger, "", 403},
		{"delete", http.MethodDelete, base, owner, "", 204},
		{"delete again", http.MethodDelete, base, owner, "", 404},
		{"bad id", http.MethodDelete, "/api/v1/playlists/nope", owner, "", 400},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if tc.tok != "" {
			req.Header.Set("Authorization", "Bearer "+tc.tok)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: %d %s", tc.name, rec.Code, rec.Body)
		}
	}
	if app.title != "Road trip" || app.order[0] != b {
		t.Fatalf("state: %q %v", app.title, app.order)
	}
}
