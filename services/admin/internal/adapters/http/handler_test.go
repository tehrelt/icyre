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
	"github.com/tehrelt/icyre/services/admin/internal/application"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

type stubRevocations struct {
	revoked map[string]bool
	err     error
}

func (s *stubRevocations) IsRevoked(_ context.Context, sid string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.revoked[sid], nil
}

type stubAdminApp struct {
	tracksPage    domain.Page[domain.TrackSummary]
	usersPage     domain.Page[domain.UserSummary]
	ingestPage    domain.Page[domain.IngestUpload]
	transcodePage domain.Page[domain.TranscodeJob]
	auditPage     domain.Page[domain.AuditEntry]
	op            domain.Operation
	err           error
}

func (s *stubAdminApp) ListTracks(context.Context, domain.Actor, domain.TrackListQuery) (domain.Page[domain.TrackSummary], error) {
	return s.tracksPage, s.err
}
func (s *stubAdminApp) ModerateTrack(context.Context, domain.Actor, string, uuid.UUID, bool, string) (domain.Operation, error) {
	return s.op, s.err
}
func (s *stubAdminApp) ListUsers(context.Context, domain.Actor, domain.UserListQuery) (domain.Page[domain.UserSummary], error) {
	return s.usersPage, s.err
}
func (s *stubAdminApp) ModerateUser(context.Context, domain.Actor, string, uuid.UUID, bool, string) (domain.Operation, error) {
	return s.op, s.err
}
func (s *stubAdminApp) ListIngestJobs(context.Context, domain.Actor, domain.IngestListQuery) (domain.Page[domain.IngestUpload], error) {
	return s.ingestPage, s.err
}
func (s *stubAdminApp) ListTranscodeJobs(context.Context, domain.Actor, domain.TranscodeJobFilter) (domain.Page[domain.TranscodeJob], error) {
	return s.transcodePage, s.err
}
func (s *stubAdminApp) ListAuditLogs(context.Context, domain.Actor, domain.AuditFilter) (domain.Page[domain.AuditEntry], error) {
	return s.auditPage, s.err
}
func (s *stubAdminApp) GetOperation(context.Context, domain.Actor, uuid.UUID) (domain.Operation, error) {
	return s.op, s.err
}

type testEnv struct {
	mux  *http.ServeMux
	priv ed25519.PrivateKey
	rev  *stubRevocations
	app  *stubAdminApp
}

func newTestEnv() *testEnv {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	rev := &stubRevocations{revoked: map[string]bool{}}
	app := &stubAdminApp{}
	v := authn.NewVerifier(authn.StaticKeys{"k1": pub}, rev)
	mux := http.NewServeMux()
	NewHandler(app, v, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	return &testEnv{mux: mux, priv: priv, rev: rev, app: app}
}

func (e *testEnv) token(t *testing.T, userID uuid.UUID, sid string, roles ...string) string {
	t.Helper()
	now := time.Now()
	c := &authn.Claims{
		SessionID: sid,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authn.Issuer,
			Audience:  jwt.ClaimStrings{authn.Audience},
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c)
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(e.priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *testEnv) do(method, path, tok, idemKey, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func TestAuthAndStrictRevocationAndRBAC(t *testing.T) {
	env := newTestEnv()
	uid := uuid.New()
	userTok := env.token(t, uid, "s-user", authn.RoleUser)
	modTok := env.token(t, uid, "s-mod", authn.RoleModerator)
	revokedTok := env.token(t, uid, "s-revoked", authn.RoleAdmin)
	env.rev.revoked["s-revoked"] = true

	// 401 missing token & revoked token.
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", "", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", revokedTok, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked token, got %d", rec.Code)
	}

	// 403 USER on moderator route, MODERATOR on admin-only routes.
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", userTok, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for USER, got %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/users", modTok, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for MODERATOR on /users, got %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/audit-logs", modTok, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for MODERATOR on /audit-logs, got %d", rec.Code)
	}

	// 503 when revocation store fails (RequiredStrict).
	env.rev.err = errors.New("redis down")
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", modTok, "", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on strict revocation failure, got %d", rec.Code)
	}
}

func TestModerateTrackAndUserResponses(t *testing.T) {
	env := newTestEnv()
	actorID := uuid.New()
	targetID := uuid.New()
	opID := uuid.New()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	modTok := env.token(t, actorID, "s1", authn.RoleModerator)
	adminTok := env.token(t, actorID, "s2", authn.RoleAdmin)

	// 200 settled.
	env.app.op = domain.Operation{
		ID: opID, ActorID: actorID, Action: domain.ActionTrackBlock, TargetType: domain.TargetTrack,
		TargetID: targetID, DesiredBlocked: true, Reason: "spam", Status: domain.StatusApplied,
		ResultCode: "APPLIED", CreatedAt: now, UpdatedAt: now, CompletedAt: &now,
	}
	rec := env.do(http.MethodPatch, "/api/v1/admin/tracks/"+targetID.String(), modTok, "k-1", `{"blocked":true,"reason":"spam"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 202 PENDING + Location header on both tracks and users, and 200 on GET /operations/{id}.
	env.app.op.Status = domain.StatusPending
	env.app.op.CompletedAt = nil
	rec = env.do(http.MethodPatch, "/api/v1/admin/tracks/"+targetID.String(), modTok, "k-2t", `{"blocked":true,"reason":"spam"}`)
	if rec.Code != http.StatusAccepted || rec.Header().Get("Location") != "/api/v1/admin/operations/"+opID.String() {
		t.Fatalf("expected 202 + Location for track, got %d loc=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	rec = env.do(http.MethodPatch, "/api/v1/admin/users/"+targetID.String(), adminTok, "k-2", `{"blocked":true,"reason":"abuse"}`)
	if rec.Code != http.StatusAccepted || rec.Header().Get("Location") != "/api/v1/admin/operations/"+opID.String() {
		t.Fatalf("expected 202 + Location, got %d loc=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/operations/"+opID.String(), modTok, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET operation, got %d: %s", rec.Code, rec.Body.String())
	}

	// 422 missing Idempotency-Key or missing blocked.
	if rec := env.do(http.MethodPatch, "/api/v1/admin/tracks/"+targetID.String(), modTok, "", `{"blocked":true,"reason":"spam"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 missing Idempotency-Key, got %d", rec.Code)
	}
	if rec := env.do(http.MethodPatch, "/api/v1/admin/tracks/"+targetID.String(), modTok, "k-3", `{"reason":"spam"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 missing blocked, got %d", rec.Code)
	}

	// 409 conflict, 422 owner error, 404 not found, 503 upstream unavailable.
	env.app.err = domain.ErrIdempotencyConflict
	if rec := env.do(http.MethodPatch, "/api/v1/admin/tracks/"+targetID.String(), modTok, "k-1", `{"blocked":false,"reason":"spam"}`); rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	env.app.err = &domain.OwnerError{Status: 422, Code: "CANNOT_BLOCK_ADMIN", Message: "Cannot block an admin account"}
	if rec := env.do(http.MethodPatch, "/api/v1/admin/users/"+targetID.String(), adminTok, "k-4", `{"blocked":true,"reason":"test"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 owner error, got %d", rec.Code)
	}
	env.app.err = domain.ErrOperationNotFound
	if rec := env.do(http.MethodGet, "/api/v1/admin/operations/"+opID.String(), modTok, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	env.app.err = domain.ErrUpstreamUnavailable
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", modTok, "", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestListEndpointsAndQueryValidation(t *testing.T) {
	env := newTestEnv()
	adminID := uuid.New()
	adminTok := env.token(t, adminID, "s-admin", authn.RoleAdmin)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	cursor := application.EncodeTimeIDCursor(now, uuid.New())

	env.app.tracksPage = domain.Page[domain.TrackSummary]{Items: []domain.TrackSummary{{ID: uuid.New(), AlbumID: uuid.New(), ArtistIDs: []uuid.UUID{uuid.New()}, Title: "T", Status: "READY", CreatedAt: now, UpdatedAt: now}}}
	env.app.usersPage = domain.Page[domain.UserSummary]{Items: []domain.UserSummary{{ID: uuid.New(), Roles: []string{"USER"}, CreatedAt: now, UpdatedAt: now}}}
	env.app.ingestPage = domain.Page[domain.IngestUpload]{Items: []domain.IngestUpload{{ID: uuid.New(), TrackID: uuid.New(), Status: "COMPLETED", CreatedAt: now, ExpiresAt: now}}}
	env.app.transcodePage = domain.Page[domain.TranscodeJob]{Items: []domain.TranscodeJob{{UploadID: uuid.New(), TrackID: uuid.New(), Status: domain.TranscodeFailed, FailureReason: "UNDECODABLE", EventAt: now, ProjectedAt: now}}}
	env.app.auditPage = domain.Page[domain.AuditEntry]{Items: []domain.AuditEntry{{ID: uuid.New(), OperationID: uuid.New(), ActorID: adminID, Action: domain.ActionUserBlock, TargetType: domain.TargetUser, TargetID: uuid.New(), Phase: domain.PhaseApplied, Reason: "abuse", CreatedAt: now}}}

	for _, path := range []string{
		"/api/v1/admin/tracks?status=READY&limit=10",
		"/api/v1/admin/users?blocked=false&limit=10",
		"/api/v1/admin/jobs?kind=ingest&status=COMPLETED&limit=10",
		"/api/v1/admin/jobs?kind=transcode&status=FAILED&limit=10&cursor=" + cursor,
		"/api/v1/admin/audit-logs?targetType=user&action=user.block&phase=applied&limit=10&cursor=" + cursor,
	} {
		rec := env.do(http.MethodGet, path, adminTok, "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("invalid JSON for %s: %v", path, err)
		}
	}

	// Missing/invalid kind on /jobs -> 422; malformed cursor -> 400.
	if rec := env.do(http.MethodGet, "/api/v1/admin/jobs", adminTok, "", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing kind, got %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/jobs?kind=transcode&cursor=bad", adminTok, "", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad cursor, got %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/audit-logs?cursor=bad", adminTok, "", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad audit cursor, got %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/v1/admin/operations/not-a-uuid", adminTok, "", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad operation id, got %d", rec.Code)
	}

	// Query validation errors -> 422.
	for _, badPath := range []string{
		"/api/v1/admin/tracks?status=UNKNOWN&limit=999",
		"/api/v1/admin/users?blocked=maybe",
		"/api/v1/admin/jobs?kind=ingest&trackId=bad&status=NOPE",
		"/api/v1/admin/jobs?kind=transcode&status=NOPE",
		"/api/v1/admin/audit-logs?targetType=album&action=bad&phase=bad",
	} {
		if rec := env.do(http.MethodGet, badPath, adminTok, "", ""); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for %s, got %d", badPath, rec.Code)
		}
	}

	// 415 unsupported media type & 400 malformed JSON.
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/tracks/"+uuid.NewString(), strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+adminTok)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
	if rec := env.do(http.MethodPatch, "/api/v1/admin/tracks/"+uuid.NewString(), adminTok, "k", `{bad`); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", rec.Code)
	}

	// 403 ErrForbidden, 422 ErrCannotBlockSelf, 503 DeadlineExceeded, 500 unexpected.
	env.app.err = domain.ErrForbidden
	if rec := env.do(http.MethodGet, "/api/v1/admin/operations/"+uuid.NewString(), adminTok, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	env.app.err = domain.ErrCannotBlockSelf
	if rec := env.do(http.MethodPatch, "/api/v1/admin/users/"+uuid.NewString(), adminTok, "k", `{"blocked":true,"reason":"self"}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "CANNOT_BLOCK_SELF") {
		t.Fatalf("expected 422 CANNOT_BLOCK_SELF, got %d body=%s", rec.Code, rec.Body.String())
	}
	env.app.err = context.DeadlineExceeded
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", adminTok, "", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 timeout, got %d", rec.Code)
	}
	env.app.err = errors.New("boom")
	if rec := env.do(http.MethodGet, "/api/v1/admin/tracks", adminTok, "", ""); rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
