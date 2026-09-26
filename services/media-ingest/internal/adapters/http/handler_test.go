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

type fakeUploads struct{ actor application.Actor }

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
