package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
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
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

type memProfiles map[uuid.UUID]domain.Profile

func (m memProfiles) Get(_ context.Context, id uuid.UUID) (domain.Profile, error) {
	p, ok := m[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return p, nil
}

func (m memProfiles) Update(_ context.Context, id uuid.UUID, c domain.Changes) (domain.Profile, error) {
	p, ok := m[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if _, err := p.Apply(c, time.Now()); err != nil {
		return domain.Profile{}, err
	}
	m[id] = p
	return p, nil
}

func setup(t *testing.T) (http.Handler, uuid.UUID, string) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	id := uuid.New()
	profiles := memProfiles{id: domain.FromRegistration(id, "rin@example.com", time.Now())}
	mux := http.NewServeMux()
	NewHandler(profiles, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)

	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s1", Roles: []string{authn.RoleUser}, RegisteredClaims: jwt.RegisteredClaims{
		Subject: id.String(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k1"
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return mux, id, signed
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPublicProfile(t *testing.T) {
	h, id, _ := setup(t)
	rec := do(h, http.MethodGet, "/api/v1/users/"+id.String(), "", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "language") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/api/v1/users/"+uuid.NewString(), "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/api/v1/users/nope", "", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}
}

func TestMe(t *testing.T) {
	h, id, token := setup(t)
	if rec := do(h, http.MethodGet, "/api/v1/users/me", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	rec := do(h, http.MethodGet, "/api/v1/users/me", token, "")
	var me ownProfile
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &me) != nil || me.ID != id.String() || me.Username != "rin" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}

	rec = do(h, http.MethodPatch, "/api/v1/users/me", token, `{"displayName":"Rin Aoki","country":"jp"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"country":"JP"`) {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodPatch, "/api/v1/users/me", token, `{"username":"-x"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "username") {
		t.Fatalf("validation: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPatch, "/api/v1/users/me", token, `{"unknown":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d", rec.Code)
	}
}
