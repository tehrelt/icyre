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

	"github.com/tehrelt/icyre/libs/platform/httpserver"
)

type errRevocations struct{ err error }

func (e errRevocations) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, e.err
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body httpserver.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body.Error.Code
}

func TestVerifyStrict(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	ctx := context.Background()
	tok := sign(t, priv, "k1", nil)

	vOK := NewVerifier(StaticKeys{"k1": pub}, revokedSet{"revoked": true})
	p, err := vOK.VerifyStrict(ctx, tok)
	if err != nil || p.UserID != "u1" {
		t.Fatalf("valid token: p=%+v err=%v", p, err)
	}
	if _, err := vOK.VerifyStrict(ctx, sign(t, priv, "k1", func(c *Claims) { c.SessionID = "revoked" })); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked token: got %v, want ErrRevoked", err)
	}
	if _, err := vOK.VerifyStrict(ctx, "not.a.jwt"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("invalid token: got %v, want ErrInvalidToken", err)
	}

	vErr := NewVerifier(StaticKeys{"k1": pub}, errRevocations{err: errors.New("redis down")})
	if _, err := vErr.Verify(ctx, tok); err != nil {
		t.Fatalf("Verify should fail open on store error, got %v", err)
	}
	if _, err := vErr.VerifyStrict(ctx, tok); !errors.Is(err, ErrRevocationUnavailable) {
		t.Fatalf("VerifyStrict: got %v, want ErrRevocationUnavailable", err)
	}
}

func TestRequiredStrict(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			t.Fatal("missing principal in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	vOK := NewVerifier(StaticKeys{"k1": pub}, revokedSet{"revoked": true})
	cases := []struct {
		name     string
		verifier *Verifier
		auth     string
		wantCode int
		wantErr  string
		wantAuth bool
	}{
		{"valid", vOK, "Bearer " + sign(t, priv, "k1", nil), http.StatusOK, "", false},
		{"missing", vOK, "", http.StatusUnauthorized, CodeUnauthenticated, true},
		{"invalid", vOK, "Bearer bad.jwt", http.StatusUnauthorized, CodeUnauthenticated, true},
		{"revoked", vOK, "Bearer " + sign(t, priv, "k1", func(c *Claims) { c.SessionID = "revoked" }), http.StatusUnauthorized, CodeUnauthenticated, true},
		{"store_down", NewVerifier(StaticKeys{"k1": pub}, errRevocations{err: errors.New("redis down")}), "Bearer " + sign(t, priv, "k1", nil), http.StatusServiceUnavailable, httpserver.CodeUnavailable, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()
			tc.verifier.RequiredStrict(okHandler).ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantErr != "" && decodeErrorCode(t, rec) != tc.wantErr {
				t.Fatalf("unexpected error code")
			}
			if tc.wantAuth && rec.Header().Get("WWW-Authenticate") != `Bearer realm="icyre"` {
				t.Fatalf("WWW-Authenticate = %q", rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRequireAnyRole(t *testing.T) {
	mw := RequireAnyRole(RoleModerator, RoleAdmin)
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name     string
		roles    []string
		hasCtx   bool
		wantCode int
		wantErr  string
	}{
		{"missing_principal", nil, false, http.StatusUnauthorized, CodeUnauthenticated},
		{"user_forbidden", []string{RoleUser}, true, http.StatusForbidden, CodeForbidden},
		{"artist_forbidden", []string{RoleArtist}, true, http.StatusForbidden, CodeForbidden},
		{"moderator_allowed", []string{RoleUser, RoleModerator}, true, http.StatusOK, ""},
		{"admin_allowed", []string{RoleAdmin}, true, http.StatusOK, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.hasCtx {
				req = req.WithContext(WithPrincipal(req.Context(), Principal{UserID: "u1", Roles: tc.roles}))
			}
			rec := httptest.NewRecorder()
			mw(okHandler).ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantErr != "" && decodeErrorCode(t, rec) != tc.wantErr {
				t.Fatalf("unexpected error code")
			}
		})
	}
}

func TestRequireInternalToken(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name     string
		expected string
		header   string
		wantCode int
		wantErr  string
	}{
		{"empty_expected_fail_closed", "", "secret", http.StatusForbidden, CodeForbidden},
		{"empty_expected_no_header", "", "", http.StatusForbidden, CodeForbidden},
		{"missing_header", "secret", "", http.StatusUnauthorized, CodeUnauthenticated},
		{"wrong_token", "secret", "wrong", http.StatusForbidden, CodeForbidden},
		{"matching_token", "secret", "secret", http.StatusOK, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set(InternalTokenHeader, tc.header)
			}
			rec := httptest.NewRecorder()
			RequireInternalToken(tc.expected)(okHandler).ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantErr != "" && decodeErrorCode(t, rec) != tc.wantErr {
				t.Fatalf("unexpected error code")
			}
		})
	}
}
