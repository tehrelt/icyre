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

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/auth/internal/application"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
	"github.com/tehrelt/icyre/services/auth/internal/ports"
)

type stubAuth struct {
	err         error
	gotRefresh  string
	users       []domain.Account
	user        domain.Account
	receipt     domain.ModerationReceipt
	gotAfterID  *uuid.UUID
	gotLimit    int
	gotBlocked  *bool
	gotOpID     uuid.UUID
	gotUserID   uuid.UUID
	gotSetBlock bool
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
func (s *stubAuth) ListModerationUsers(_ context.Context, afterID *uuid.UUID, limit int, blocked *bool) ([]domain.Account, error) {
	s.gotAfterID = afterID
	s.gotLimit = limit
	s.gotBlocked = blocked
	return s.users, s.err
}
func (s *stubAuth) GetModerationUser(_ context.Context, userID uuid.UUID) (domain.Account, error) {
	s.gotUserID = userID
	return s.user, s.err
}
func (s *stubAuth) ModerateBlock(_ context.Context, operationID, userID uuid.UUID, blocked bool) (domain.ModerationReceipt, error) {
	s.gotOpID = operationID
	s.gotUserID = userID
	s.gotSetBlock = blocked
	return s.receipt, s.err
}

const testInternalToken = "test-internal-secret"

func newServer(app Auth) http.Handler {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	v := authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil)
	mux := http.NewServeMux()
	NewHandler(app, v, authn.JWKS{Keys: []authn.JWK{authn.PublicJWK("k1", pub)}}, Options{
		SecureCookies: true,
		InternalToken: testInternalToken,
	}, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
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

func doInternal(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if token != "" {
		req.Header.Set(authn.InternalTokenHeader, token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestInternalModerationAuth(t *testing.T) {
	h := newServer(&stubAuth{})
	uid := uuid.New().String()
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/internal/v1/moderation/users", ""},
		{http.MethodGet, "/internal/v1/moderation/users/" + uid, ""},
		{http.MethodPatch, "/internal/v1/moderation/users/" + uid, `{"operationId":"` + uuid.New().String() + `","blocked":true}`},
	}
	for _, rt := range routes {
		if rec := doInternal(h, rt.method, rt.path, rt.body, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s missing token: got %d, want 401", rt.method, rt.path, rec.Code)
		}
		if rec := doInternal(h, rt.method, rt.path, rt.body, "wrong-token"); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s wrong token: got %d, want 403", rt.method, rt.path, rec.Code)
		}
	}
}

func TestInternalListModerationUsers(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	u1 := domain.Account{ID: uuid.MustParse("01900000-0000-7000-8000-000000000001"), Email: "secret1@example.com", PasswordHash: "$argon2id$secret1", Roles: []string{domain.RoleUser}, CreatedAt: now, UpdatedAt: now}
	u2 := domain.Account{ID: uuid.MustParse("01900000-0000-7000-8000-000000000002"), Email: "secret2@example.com", PasswordHash: "$argon2id$secret2", Roles: []string{domain.RoleUser}, Blocked: true, BlockedAt: &now, CreatedAt: now, UpdatedAt: now}
	u3 := domain.Account{ID: uuid.MustParse("01900000-0000-7000-8000-000000000003"), Email: "secret3@example.com", PasswordHash: "$argon2id$secret3", Roles: []string{domain.RoleUser}, CreatedAt: now, UpdatedAt: now}

	stub := &stubAuth{users: []domain.Account{u1, u2, u3}}
	cursor := uuid.MustParse("01900000-0000-7000-8000-000000000000")
	rec := doInternal(newServer(stub), http.MethodGet, "/internal/v1/moderation/users?limit=2&blocked=true&cursor="+cursor.String(), "", testInternalToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if stub.gotLimit != 3 || stub.gotAfterID == nil || *stub.gotAfterID != cursor || stub.gotBlocked == nil || !*stub.gotBlocked {
		t.Fatalf("unexpected query args: after=%v limit=%d blocked=%v", stub.gotAfterID, stub.gotLimit, stub.gotBlocked)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"secret1@example.com", "secret2@example.com", "$argon2id$", "passwordHash", "email"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("sensitive field %q leaked in response: %s", forbidden, body)
		}
	}
	var parsed struct {
		Data       []moderationUserDTO `json:"data"`
		Pagination paginationDTO       `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Data) != 2 || !parsed.Pagination.HasMore || parsed.Pagination.NextCursor == nil || *parsed.Pagination.NextCursor != u2.ID.String() {
		t.Fatalf("unexpected response: %+v", parsed)
	}
}

func TestInternalGetAndPatchModerationUser(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	uid := uuid.New()
	opID := uuid.New()
	stub := &stubAuth{
		user: domain.Account{ID: uid, Email: "hidden@example.com", PasswordHash: "$argon2id$hidden", Roles: []string{domain.RoleUser}, Blocked: true, BlockedAt: &now, CreatedAt: now, UpdatedAt: now},
		receipt: domain.ModerationReceipt{
			OperationID: opID, UserID: uid, Blocked: true, Changed: true, RevokedSessions: 2, RevocationsPending: true, CreatedAt: now,
		},
	}
	h := newServer(stub)

	getRec := doInternal(h, http.MethodGet, "/internal/v1/moderation/users/"+uid.String(), "", testInternalToken)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", getRec.Code, getRec.Body)
	}
	if strings.Contains(getRec.Body.String(), "hidden@example.com") || strings.Contains(getRec.Body.String(), "$argon2id$") {
		t.Fatalf("sensitive data leaked: %s", getRec.Body)
	}

	patchRec := doInternal(h, http.MethodPatch, "/internal/v1/moderation/users/"+uid.String(), `{"operationId":"`+opID.String()+`","blocked":true}`, testInternalToken)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", patchRec.Code, patchRec.Body)
	}
	var got moderationReceiptDTO
	if err := json.Unmarshal(patchRec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.OperationID != opID.String() || got.UserID != uid.String() || !got.Blocked || !got.Changed || got.RevokedSessions != 2 || !got.RevocationsPending {
		t.Fatalf("unexpected receipt DTO: %+v", got)
	}

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrOperationConflict, http.StatusConflict, "OPERATION_CONFLICT"},
		{domain.ErrCannotBlockAdmin, http.StatusUnprocessableEntity, "CANNOT_BLOCK_ADMIN"},
		{domain.ErrAccountNotFound, http.StatusNotFound, "ACCOUNT_NOT_FOUND"},
	}
	for _, c := range cases {
		errStub := &stubAuth{err: c.err}
		rec := doInternal(newServer(errStub), http.MethodPatch, "/internal/v1/moderation/users/"+uid.String(), `{"operationId":"`+opID.String()+`","blocked":true}`, testInternalToken)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%v: got %d %s, want %d %s", c.err, rec.Code, rec.Body, c.status, c.code)
		}
	}
}
