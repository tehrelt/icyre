// Package http serves profiles: public GET /users/{id}, owner /users/me.
package http

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/user-profile/internal/application"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

// Profiles is what the handlers need from the application layer.
type Profiles interface {
	Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	Update(ctx context.Context, userID uuid.UUID, c domain.Changes) (domain.Profile, error)
	CreateAvatarUpload(ctx context.Context, userID uuid.UUID, req domain.AvatarUploadRequest) (application.CreatedAvatarUpload, error)
	CompleteAvatarUpload(ctx context.Context, userID, uploadID uuid.UUID) (domain.Profile, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	AvatarURL(ctx context.Context, avatarKey string) (*string, error)
}

// Handler serves profile routes.
type Handler struct {
	app      Profiles
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Profiles, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes. "me" is matched before {id}.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/users/me", h.verifier.Required(http.HandlerFunc(h.me)))
	mux.Handle("PATCH /api/v1/users/me", h.verifier.Required(http.HandlerFunc(h.updateMe)))
	mux.Handle("POST /api/v1/users/me/avatar/uploads", h.verifier.Required(http.HandlerFunc(h.createAvatarUpload)))
	mux.Handle("POST /api/v1/users/me/avatar/uploads/{id}/complete", h.verifier.Required(http.HandlerFunc(h.completeAvatarUpload)))
	mux.Handle("DELETE /api/v1/users/me/avatar", h.verifier.Required(http.HandlerFunc(h.deleteAvatar)))
	mux.HandleFunc("GET /api/v1/users/{id}", h.get)
}

// publicProfile is visible to anyone.
type publicProfile struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"`
	Bio         string  `json:"bio"`
	Country     string  `json:"country"`
}

// ownProfile adds private settings.
type ownProfile struct {
	publicProfile
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type updateRequest struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"displayName"`
	Bio         *string `json:"bio"`
	Country     *string `json:"country"`
	Language    *string `json:"language"`
}

type createAvatarUploadRequest struct {
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
}

type signedURLView struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type avatarUploadView struct {
	ID          string                    `json:"id"`
	Status      domain.AvatarUploadStatus `json:"status"`
	ContentType string                    `json:"contentType"`
	SizeBytes   int64                     `json:"sizeBytes"`
	SHA256      string                    `json:"sha256"`
	CreatedAt   time.Time                 `json:"createdAt"`
	ExpiresAt   time.Time                 `json:"expiresAt"`
	Upload      signedURLView             `json:"upload"`
}

func (h *Handler) toPublic(ctx context.Context, p domain.Profile) publicProfile {
	avatarURL, _ := h.app.AvatarURL(ctx, p.AvatarKey)
	return publicProfile{
		ID:          p.UserID.String(),
		Username:    p.Username,
		DisplayName: p.DisplayName,
		AvatarURL:   avatarURL,
		Bio:         p.Bio,
		Country:     p.Country,
	}
}

func (h *Handler) toOwn(ctx context.Context, p domain.Profile) ownProfile {
	return ownProfile{
		publicProfile: h.toPublic(ctx, p),
		Language:      p.Language,
		CreatedAt:     p.CreatedAt.UTC(),
		UpdatedAt:     p.UpdatedAt.UTC(),
	}
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter id must be a UUID", nil)
		return
	}
	p, err := h.app.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, h.toPublic(r.Context(), p))
}

func principalID(r *http.Request) (uuid.UUID, bool) {
	p, ok := authn.FromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(p.UserID)
	return id, err == nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	p, err := h.app.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	httpserver.WriteJSON(w, http.StatusOK, h.toOwn(r.Context(), p))
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	id, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req updateRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	p, err := h.app.Update(r.Context(), id, domain.Changes(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, h.toOwn(r.Context(), p))
}

func (h *Handler) createAvatarUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req createAvatarUploadRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	c, err := h.app.CreateAvatarUpload(r.Context(), id, domain.AvatarUploadRequest{
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		SHA256Hex:   req.SHA256,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v := avatarUploadView{
		ID:          c.Upload.ID.String(),
		Status:      c.Upload.Status,
		ContentType: c.Upload.ContentType,
		SizeBytes:   c.Upload.SizeBytes,
		SHA256:      hex.EncodeToString(c.Upload.SHA256),
		CreatedAt:   c.Upload.CreatedAt.UTC(),
		ExpiresAt:   c.Upload.ExpiresAt.UTC(),
		Upload: signedURLView{
			Method:    c.URL.Method,
			URL:       c.URL.URL,
			Headers:   c.URL.Headers,
			ExpiresAt: c.URL.ExpiresAt.UTC(),
		},
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Location", "/api/v1/users/me/avatar/uploads/"+c.Upload.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, v)
}

func (h *Handler) completeAvatarUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	uploadID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter id must be a UUID", nil)
		return
	}
	p, err := h.app.CompleteAvatarUpload(r.Context(), userID, uploadID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusOK, h.toOwn(r.Context(), p))
}

func (h *Handler) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	p, err := h.app.DeleteAvatar(r.Context(), userID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusOK, h.toOwn(r.Context(), p))
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ve *domain.ValidationError
		re *domain.AvatarRejectedError
	)
	switch {
	case errors.As(err, &ve):
		fields := map[string]any{}
		for k, v := range ve.Fields {
			fields[k] = v
		}
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
	case errors.As(err, &re):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, "UPLOAD_REJECTED", "The uploaded file failed verification", map[string]any{"reason": re.Reason})
	case errors.Is(err, domain.ErrUsernameTaken):
		httpserver.WriteError(w, r, http.StatusConflict, "USERNAME_TAKEN", "This username is taken", nil)
	case errors.Is(err, domain.ErrProfileNotFound):
		// Right after sign-up the profile may still be on its way from auth.events.
		httpserver.WriteError(w, r, http.StatusNotFound, "PROFILE_NOT_FOUND", "Profile not found", nil)
	case errors.Is(err, domain.ErrAvatarUploadNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "UPLOAD_NOT_FOUND", "Upload not found", nil)
	case errors.Is(err, domain.ErrAvatarNotUploaded):
		httpserver.WriteError(w, r, http.StatusConflict, "UPLOAD_INCOMPLETE", "The file has not been uploaded yet", nil)
	case errors.Is(err, domain.ErrStorageUnavailable):
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Avatar storage is temporarily unavailable", nil)
	default:
		h.log.ErrorContext(r.Context(), "profile request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, httpserver.CodeInternal, "Internal server error", nil)
	}
}
