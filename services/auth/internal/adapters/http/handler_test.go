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

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/auth/internal/application"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
	"github.com/tehrelt/icyre/services/auth/internal/ports"
)

type stubAuth struct {
	err        error
	gotRefresh string
}

func result() application.Result {
	acc := domain.NewAccount(uuid.New(), "rin@example.com", "x", time.Now())
	return application.Result{
		Account: acc, SessionID: uuid.New(),
		Access:       ports.AccessToken{Token: "jwt", ExpiresAt: time.Now().Add(15 * time.Minute)},
		RefreshToken: "refresh-1", RefreshUntil: time.Now().Add(24 * time.Hour),
	}
}

func (s *stubAuth) Register(context.Context, string, string, application.Client) (application.Result, error) {
	return result(), s.err
}
func (s *stubAuth) Login(context.Context, string, string, application.Client) (application.Result, error) {
	return result(), s.err
}
func (s *stubAuth) Refresh(_ context.Context, t string) (application.Result, error) {
	s.gotRefresh = t
	return result(), s.err
}
func (s *stubAuth) Logout(_ context.Context, t string) error { s.gotRefresh = t; return s.err }
func (s *stubAuth) Sessions(context.Context, uuid.UUID) ([]domain.Session, error) {
	return nil, s.err
}
func (s *stubAuth) RevokeSession(context.Context, uuid.UUID, uuid.UUID) error { return s.err }

func newServer(app Auth) http.Handler {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	v := authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil)
	mux := http.NewServeMux()
	NewHandler(app, v, authn.JWKS{Keys: []authn.JWK{authn.PublicJWK("k1", pub)}}, Options{SecureCookies: true}, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	return mux
}

func do(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegisterSetsHardenedRefreshCookie(t *testing.T) {
	rec := do(newServer(&stubAuth{}), http.MethodPost, "/api/v1/auth/register", `{"email":"rin@example.com","password":"correct horse battery"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	c := rec.Result().Cookies()[0]
	if c.Name != RefreshCookie || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/api/v1/auth" {
		t.Fatalf("cookie = %+v", c)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"accessToken":"jwt"`) || strings.Contains(body, "refresh-1") {
		t.Fatalf("refresh token must not appear in the body: %s", body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("token responses must not be cached")
	}
}

func TestRefreshReadsCookie(t *testing.T) {
	stub := &stubAuth{}
	rec := do(newServer(stub), http.MethodPost, "/api/v1/auth/refresh", "", &http.Cookie{Name: RefreshCookie, Value: "abc"})
	if rec.Code != http.StatusOK || stub.gotRefresh != "abc" {
		t.Fatalf("status %d token %q", rec.Code, stub.gotRefresh)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		path   string
		status int
		code   string
	}{
		{domain.ErrEmailTaken, "/api/v1/auth/register", 409, "EMAIL_TAKEN"},
		{&domain.ValidationError{Fields: map[string]string{"email": "bad"}}, "/api/v1/auth/register", 422, "VALIDATION_FAILED"},
		{domain.ErrInvalidCredentials, "/api/v1/auth/login", 401, "INVALID_CREDENTIALS"},
		{domain.ErrTooManyAttempts, "/api/v1/auth/login", 429, "TOO_MANY_ATTEMPTS"},
		{domain.ErrRefreshReuse, "/api/v1/auth/refresh", 401, "SESSION_EXPIRED"},
	}
	for _, c := range cases {
		rec := do(newServer(&stubAuth{err: c.err}), http.MethodPost, c.path, `{"email":"a@b.co","password":"x"}`, nil)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%v: %d %s", c.err, rec.Code, rec.Body)
		}
	}
	// A failed refresh clears the stale cookie.
	rec := do(newServer(&stubAuth{err: domain.ErrSessionInactive}), http.MethodPost, "/api/v1/auth/refresh", "", &http.Cookie{Name: RefreshCookie, Value: "old"})
	if c := rec.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Fatalf("stale cookie not cleared: %+v", c)
	}
}

func TestSessionsRequireAccessTokenAndJWKSIsPublic(t *testing.T) {
	h := newServer(&stubAuth{})
	if rec := do(h, http.MethodGet, "/api/v1/auth/sessions", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sessions without token = %d", rec.Code)
	}
	rec := do(h, http.MethodGet, "/api/v1/auth/.well-known/jwks.json", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"crv":"Ed25519"`) {
		t.Fatalf("jwks = %d %s", rec.Code, rec.Body)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	stub := &stubAuth{}
	rec := do(newServer(stub), http.MethodPost, "/api/v1/auth/logout", "", &http.Cookie{Name: RefreshCookie, Value: "abc"})
	if rec.Code != http.StatusNoContent || stub.gotRefresh != "abc" || rec.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout: %d %q", rec.Code, stub.gotRefresh)
	}
}
