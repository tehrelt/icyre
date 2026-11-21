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
	"github.com/tehrelt/icyre/services/user-profile/internal/application"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

var (
	knownUploadID       = uuid.MustParse("0192a000-0000-7000-8000-000000000001")
	rejectedUploadID    = uuid.MustParse("0192a000-0000-7000-8000-000000000002")
	incompleteUploadID  = uuid.MustParse("0192a000-0000-7000-8000-000000000003")
	unavailableUploadID = uuid.MustParse("0192a000-0000-7000-8000-000000000004")
)

type memProfiles struct {
	profiles   map[uuid.UUID]domain.Profile
	storageOff bool
}

func (m *memProfiles) Get(_ context.Context, id uuid.UUID) (domain.Profile, error) {
	p, ok := m.profiles[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return p, nil
}

func (m *memProfiles) Update(_ context.Context, id uuid.UUID, c domain.Changes) (domain.Profile, error) {
	p, ok := m.profiles[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if _, err := p.Apply(c, time.Now()); err != nil {
		return domain.Profile{}, err
	}
	m.profiles[id] = p
	return p, nil
}

func (m *memProfiles) CreateAvatarUpload(_ context.Context, id uuid.UUID, req domain.AvatarUploadRequest) (application.CreatedAvatarUpload, error) {
	if m.storageOff {
		return application.CreatedAvatarUpload{}, domain.ErrStorageUnavailable
	}
	if _, ok := m.profiles[id]; !ok {
		return application.CreatedAvatarUpload{}, domain.ErrProfileNotFound
	}
	f, sum, err := req.Validate(5 << 20)
	if err != nil {
		return application.CreatedAvatarUpload{}, err
	}
	now := time.Now().UTC()
	return application.CreatedAvatarUpload{
		Upload: domain.AvatarUpload{
			ID:          knownUploadID,
			UserID:      id,
			ContentType: f.ContentType,
			SizeBytes:   req.SizeBytes,
			SHA256:      sum,
			Status:      domain.AvatarStatusPending,
			CreatedAt:   now,
			ExpiresAt:   now.Add(15 * time.Minute),
		},
		URL: objectstore.SignedURL{
			Method:    http.MethodPut,
			URL:       "http://minio/users/" + id.String() + "/avatars/" + knownUploadID.String() + "." + f.Ext,
			Headers:   map[string]string{"Content-Type": f.ContentType},
			ExpiresAt: now.Add(15 * time.Minute),
		},
	}, nil
}

func (m *memProfiles) CompleteAvatarUpload(_ context.Context, userID, uploadID uuid.UUID) (domain.Profile, error) {
	switch uploadID {
	case knownUploadID:
		p, ok := m.profiles[userID]
		if !ok {
			return domain.Profile{}, domain.ErrProfileNotFound
		}
		p.AvatarKey = "users/" + userID.String() + "/avatars/" + uploadID.String() + ".png"
		m.profiles[userID] = p
		return p, nil
	case rejectedUploadID:
		return domain.Profile{}, &domain.AvatarRejectedError{Reason: domain.ReasonSizeMismatch}
	case incompleteUploadID:
		return domain.Profile{}, domain.ErrAvatarNotUploaded
	case unavailableUploadID:
		return domain.Profile{}, domain.ErrStorageUnavailable
	default:
		return domain.Profile{}, domain.ErrAvatarUploadNotFound
	}
}

func (m *memProfiles) DeleteAvatar(_ context.Context, userID uuid.UUID) (domain.Profile, error) {
	p, ok := m.profiles[userID]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	p.AvatarKey = ""
	m.profiles[userID] = p
	return p, nil
}

func (m *memProfiles) AvatarURL(_ context.Context, avatarKey string) (*string, error) {
	if avatarKey == "" {
		return nil, nil
	}
	u := "http://minio/" + avatarKey + "?signed=1"
	return &u, nil
}

func setup(t *testing.T) (http.Handler, *memProfiles, uuid.UUID, string) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	id := uuid.New()
	profiles := &memProfiles{profiles: map[uuid.UUID]domain.Profile{
		id: domain.FromRegistration(id, "rin@example.com", time.Now()),
	}}
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
	return mux, profiles, id, signed
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
	h, _, id, _ := setup(t)
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
	h, _, id, token := setup(t)
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

func TestAvatarEndpoints(t *testing.T) {
	h, profiles, id, token := setup(t)
	validBody := `{"contentType":"image/png","sizeBytes":1024,"sha256":"` + strings.Repeat("ab", 32) + `"}`

	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads", "", validBody); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads", token, `{"unknown":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads", token, `{"contentType":"image/gif","sizeBytes":0,"sha256":"bad"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validation: %d %s", rec.Code, rec.Body)
	}

	profiles.storageOff = true
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads", token, validBody); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("storage off: %d %s", rec.Code, rec.Body)
	}
	profiles.storageOff = false

	rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads", token, validBody)
	var created avatarUploadView
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil ||
		rec.Header().Get("Location") != "/api/v1/users/me/avatar/uploads/"+knownUploadID.String() ||
		created.Upload.Method != http.MethodPut {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}

	// Complete errors.
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads/nope/complete", token, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad upload id: %d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads/"+uuid.NewString()+"/complete", token, ""); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "UPLOAD_NOT_FOUND") {
		t.Fatalf("not found: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads/"+incompleteUploadID.String()+"/complete", token, ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "UPLOAD_INCOMPLETE") {
		t.Fatalf("incomplete: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads/"+rejectedUploadID.String()+"/complete", token, ""); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "UPLOAD_REJECTED") {
		t.Fatalf("rejected: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads/"+unavailableUploadID.String()+"/complete", token, ""); rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Fatalf("unavailable: %d %s", rec.Code, rec.Body)
	}

	// Complete success populates avatarUrl across complete, me, patch, and public get.
	rec = do(h, http.MethodPost, "/api/v1/users/me/avatar/uploads/"+knownUploadID.String()+"/complete", token, "")
	var own ownProfile
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &own) != nil || own.AvatarURL == nil || !strings.Contains(*own.AvatarURL, knownUploadID.String()) {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodGet, "/api/v1/users/"+id.String(), "", "")
	var pub publicProfile
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &pub) != nil || pub.AvatarURL == nil {
		t.Fatalf("public get with avatar: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodGet, "/api/v1/users/me", token, "")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &own) != nil || own.AvatarURL == nil {
		t.Fatalf("me with avatar: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodPatch, "/api/v1/users/me", token, `{"bio":"hi"}`)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &own) != nil || own.AvatarURL == nil {
		t.Fatalf("patch with avatar: %d %s", rec.Code, rec.Body)
	}

	// Delete avatar clears avatarUrl.
	if rec := do(h, http.MethodDelete, "/api/v1/users/me/avatar", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete: %d", rec.Code)
	}
	rec = do(h, http.MethodDelete, "/api/v1/users/me/avatar", token, "")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &own) != nil || own.AvatarURL != nil {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	_ = errors.New
}
