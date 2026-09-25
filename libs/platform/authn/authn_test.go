package authn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func sign(t *testing.T, priv ed25519.PrivateKey, kid string, mutate func(*Claims)) string {
	t.Helper()
	now := time.Now()
	c := &Claims{
		SessionID: "s1",
		Roles:     []string{RoleUser},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{Audience},
			Subject:   "u1",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	if mutate != nil {
		mutate(c)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type revokedSet map[string]bool

func (r revokedSet) IsRevoked(_ context.Context, sid string) (bool, error) { return r[sid], nil }

func TestVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	v := NewVerifier(StaticKeys{"k1": pub}, revokedSet{"revoked": true})
	ctx := context.Background()

	p, err := v.Verify(ctx, sign(t, priv, "k1", nil))
	if err != nil || p.UserID != "u1" || p.SessionID != "s1" || !p.HasRole(RoleUser) {
		t.Fatalf("p = %+v, err = %v", p, err)
	}

	bad := map[string]string{
		"expired":      sign(t, priv, "k1", func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }),
		"wrong issuer": sign(t, priv, "k1", func(c *Claims) { c.Issuer = "evil" }),
		"wrong aud":    sign(t, priv, "k1", func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} }),
		"wrong key":    sign(t, otherPriv, "k1", nil),
		"unknown kid":  sign(t, priv, "k9", nil),
		"no sid":       sign(t, priv, "k1", func(c *Claims) { c.SessionID = "" }),
		"garbage":      "not.a.jwt",
	}
	for name, tok := range bad {
		if _, err := v.Verify(ctx, tok); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: expected ErrInvalidToken, got %v", name, err)
		}
	}

	// alg=none / HS256 must never be accepted.
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u1", "sid": "s1", "iss": Issuer, "aud": Audience, "exp": time.Now().Add(time.Hour).Unix()})
	hs.Header["kid"] = "k1"
	hsTok, _ := hs.SignedString([]byte(pub))
	if _, err := v.Verify(ctx, hsTok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("HS256 token accepted: %v", err)
	}

	if _, err := v.Verify(ctx, sign(t, priv, "k1", func(c *Claims) { c.SessionID = "revoked" })); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked session accepted: %v", err)
	}
}

func TestMiddleware(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	v := NewVerifier(StaticKeys{"k1": pub}, nil)
	var seen *Principal
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := FromContext(r.Context()); ok {
			seen = &p
		}
	})

	do := func(mw func(http.Handler) http.Handler, auth string) int {
		seen = nil
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		mw(h).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do(v.Required, ""); code != http.StatusUnauthorized {
		t.Fatalf("required without token = %d", code)
	}
	if code := do(v.Required, "Bearer "+sign(t, priv, "k1", nil)); code != http.StatusOK || seen == nil || seen.UserID != "u1" {
		t.Fatalf("required with token = %d, principal %+v", code, seen)
	}
	if code := do(v.Optional, ""); code != http.StatusOK || seen != nil {
		t.Fatalf("optional anonymous = %d", code)
	}
	if code := do(v.Optional, "Bearer junk"); code != http.StatusUnauthorized {
		t.Fatalf("optional with junk token = %d", code)
	}
	if code := do(v.Required, "Basic dTpw"); code != http.StatusUnauthorized {
		t.Fatalf("basic auth = %d", code)
	}
}

func TestRemoteKeys(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(JWKS{Keys: []JWK{PublicJWK("k1", pub)}})
	}))
	defer srv.Close()

	v := NewVerifier(NewRemoteKeys(srv.URL, srv.Client()), nil)
	for i := 0; i < 3; i++ {
		if _, err := v.Verify(context.Background(), sign(t, priv, "k1", nil)); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("jwks fetched %d times", calls)
	}
	// An unknown kid within minRefresh does not hammer the endpoint.
	if _, err := v.Verify(context.Background(), sign(t, priv, "k2", nil)); err == nil {
		t.Fatal("unknown kid accepted")
	}
	if calls != 1 {
		t.Fatalf("jwks refetched too eagerly: %d", calls)
	}
}
