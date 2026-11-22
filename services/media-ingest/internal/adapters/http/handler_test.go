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
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/services/media-ingest/internal/application"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

type fakeUploads struct {
	actor        application.Actor
	listResult   []domain.Upload
	listErr      error
	gotTrackID   *uuid.UUID
	gotStatus    *domain.Status
	gotAfterTime *time.Time
	gotAfterID   *uuid.UUID
	gotLimit     int
}

var known = uuid.MustParse("0192a000-0000-7000-8000-000000000001")

func (f *fakeUploads) Create(_ context.Context, a application.Actor, req domain.Request) (application.Created, error) {
	f.actor = a
	if !a.Artist && !a.Admin {
		return application.Created{}, domain.ErrForbidden
	}
	if _, _, err := req.Validate(1 << 20); err != nil {
		return application.Created{}, err
	}
	return application.Created{
		Upload: domain.Upload{ID: known, TrackID: req.TrackID, Status: domain.StatusPending, ContentType: req.ContentType, SizeBytes: req.SizeBytes},
		URL:    objectstore.SignedURL{Method: "PUT", URL: "http://minio/x", Headers: map[string]string{"Content-Type": req.ContentType}},
	}, nil
}
func (f *fakeUploads) Get(_ context.Context, _ application.Actor, id uuid.UUID) (domain.Upload, error) {
	if id != known {
		return domain.Upload{}, domain.ErrNotFound
	}
	return domain.Upload{ID: id, Status: domain.StatusPending}, nil
}
func (f *fakeUploads) Complete(_ context.Context, _ application.Actor, id uuid.UUID) (domain.Upload, error) {
	switch id {
	case known:
		return domain.Upload{ID: id, Status: domain.StatusFailed, FailureReason: "SIZE_MISMATCH"}, &domain.RejectedError{Reason: "SIZE_MISMATCH"}
	case uuid.Nil:
		return domain.Upload{}, domain.ErrNotUploaded
	}
	return domain.Upload{}, errors.New("db down")
}
func (f *fakeUploads) ListModerationUploads(_ context.Context, trackID *uuid.UUID, status *domain.Status, afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]domain.Upload, error) {
	f.gotTrackID = trackID
	f.gotStatus = status
	f.gotAfterTime = afterCreatedAt
	f.gotAfterID = afterID
	f.gotLimit = limit
	return f.listResult, f.listErr
}

func setup(t *testing.T) (http.Handler, *fakeUploads, func(roles ...string) string) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	app := &fakeUploads{}
	mux := http.NewServeMux()
	NewHandler(app, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	token := func(roles ...string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s1", Roles: roles, RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.NewString(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		}})
		tok.Header["kid"] = "k1"
		s, err := tok.SignedString(priv)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return mux, app, token
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadAPI(t *testing.T) {
	h, app, token := setup(t)
	body := `{"trackId":"` + uuid.NewString() + `","contentType":"audio/flac","sizeBytes":1000,"sha256":"` + strings.Repeat("ab", 32) + `"}`
	if rec := do(h, http.MethodPost, "/api/v1/media/uploads", "", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/media/uploads", token(authn.RoleUser), body); rec.Code != http.StatusForbidden {
		t.Fatalf("listener: %d", rec.Code)
	}
	rec := do(h, http.MethodPost, "/api/v1/media/uploads", token(authn.RoleUser, authn.RoleArtist), body)
	var v uploadView
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &v) != nil || v.Upload == nil || v.Upload.Method != "PUT" ||
		v.Status != domain.StatusPending || rec.Header().Get("Location") != "/api/v1/media/uploads/"+known.String() || !app.actor.Artist {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/media/uploads", token(authn.RoleAdmin), `{"trackId":"x","contentType":"video/mp4","sizeBytes":0,"sha256":""}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/media/uploads", token(authn.RoleAdmin), `{"bogus":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d", rec.Code)
	}

	tok := token(authn.RoleArtist)
	for path, want := range map[string]int{
		"/api/v1/media/uploads/" + known.String():   http.StatusOK,
		"/api/v1/media/uploads/" + uuid.NewString(): http.StatusNotFound,
		"/api/v1/media/uploads/nope":                http.StatusBadRequest,
	} {
		if rec := do(h, http.MethodGet, path, tok, ""); rec.Code != want || rec.Header().Get("Cache-Control") != "private, no-store" && want != http.StatusBadRequest {
			t.Errorf("GET %s: %d want %d", path, rec.Code, want)
		}
	}
	rec = do(h, http.MethodPost, "/api/v1/media/uploads/"+known.String()+"/complete", tok, "")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"reason":"SIZE_MISMATCH"`) {
		t.Fatalf("rejected: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/media/uploads/"+uuid.Nil.String()+"/complete", tok, ""); rec.Code != http.StatusConflict {
		t.Fatalf("not uploaded: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/media/uploads/"+uuid.NewString()+"/complete", tok, ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("down: %d", rec.Code)
	}
}

func TestInternalModerationUploads(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	app := &fakeUploads{}
	mux := http.NewServeMux()
	h := NewHandler(app, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.Register(mux)
	h.RegisterInternal(mux, "secret-token")

	doInternal := func(token, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set(authn.InternalTokenHeader, token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	if rec := doInternal("", "/internal/v1/moderation/uploads"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: %d", rec.Code)
	}
	if rec := doInternal("wrong", "/internal/v1/moderation/uploads"); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong token: %d", rec.Code)
	}

	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(time.Minute)
	trackID := uuid.Must(uuid.NewV7())
	uploaderID := uuid.Must(uuid.NewV7())
	u1 := domain.Upload{
		ID: uuid.Must(uuid.NewV7()), TrackID: trackID, UploaderID: uploaderID,
		ContentType: "audio/flac", SizeBytes: 1024, SHA256: []byte("0123456789abcdef0123456789abcdef"),
		Key: "tracks/secret/original/u1.flac", Status: domain.StatusFailed, FailureReason: "SIZE_MISMATCH",
		CreatedAt: now, ExpiresAt: now.Add(time.Hour), CompletedAt: &completedAt,
	}
	u2 := domain.Upload{
		ID: uuid.Must(uuid.NewV7()), TrackID: trackID, UploaderID: uploaderID,
		Key: "tracks/secret/original/u2.flac", Status: domain.StatusPending,
		CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	u3 := domain.Upload{
		ID: uuid.Must(uuid.NewV7()), TrackID: trackID, UploaderID: uploaderID,
		Key: "tracks/secret/original/u3.flac", Status: domain.StatusCompleted,
		CreatedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(time.Hour), CompletedAt: &completedAt,
	}
	app.listResult = []domain.Upload{u1, u2, u3}

	rec := doInternal("secret-token", "/internal/v1/moderation/uploads?trackId="+trackID.String()+"&status=failed&limit=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if app.gotTrackID == nil || *app.gotTrackID != trackID || app.gotStatus == nil || *app.gotStatus != domain.StatusFailed || app.gotLimit != 3 {
		t.Fatalf("query args: track=%v status=%v limit=%d", app.gotTrackID, app.gotStatus, app.gotLimit)
	}

	raw := rec.Body.String()
	for _, forbidden := range []string{"objectKey", "sha256", "uploaderId", uploaderID.String(), "tracks/secret", `"upload":`} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("response leaked internal field %q: %s", forbidden, raw)
		}
	}

	var resp moderationUploadsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 || !resp.Pagination.HasMore || resp.Pagination.NextCursor == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Data[0].ID != u1.ID.String() || resp.Data[0].FailureReason == nil || *resp.Data[0].FailureReason != "SIZE_MISMATCH" || resp.Data[0].CompletedAt == nil {
		t.Fatalf("unexpected item[0]: %+v", resp.Data[0])
	}

	// Cursor round-trip.
	app.listResult = []domain.Upload{u3}
	rec = doInternal("secret-token", "/internal/v1/moderation/uploads?cursor="+*resp.Pagination.NextCursor)
	if rec.Code != http.StatusOK {
		t.Fatalf("cursor status = %d: %s", rec.Code, rec.Body)
	}
	if app.gotAfterTime == nil || !app.gotAfterTime.Equal(u2.CreatedAt) || app.gotAfterID == nil || *app.gotAfterID != u2.ID {
		t.Fatalf("cursor mismatch: time=%v id=%v", app.gotAfterTime, app.gotAfterID)
	}

	// Validation and malformed cursor.
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/internal/v1/moderation/uploads?trackId=nope", http.StatusUnprocessableEntity},
		{"/internal/v1/moderation/uploads?status=BOGUS", http.StatusUnprocessableEntity},
		{"/internal/v1/moderation/uploads?limit=101", http.StatusUnprocessableEntity},
		{"/internal/v1/moderation/uploads?limit=0", http.StatusUnprocessableEntity},
		{"/internal/v1/moderation/uploads?cursor=bad!", http.StatusBadRequest},
	} {
		if r := doInternal("secret-token", tc.path); r.Code != tc.want {
			t.Errorf("%s: status = %d, want %d: %s", tc.path, r.Code, tc.want, r.Body)
		}
	}
}
