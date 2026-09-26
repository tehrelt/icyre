// Package http serves upload sessions under /api/v1/media/uploads.
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
	"github.com/tehrelt/icyre/services/media-ingest/internal/application"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

// Uploads is the application layer.
type Uploads interface {
	Create(ctx context.Context, actor application.Actor, req domain.Request) (application.Created, error)
	Get(ctx context.Context, actor application.Actor, id uuid.UUID) (domain.Upload, error)
	Complete(ctx context.Context, actor application.Actor, id uuid.UUID) (domain.Upload, error)
}

// Handler serves the upload API.
type Handler struct {
	app      Uploads
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Uploads, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes (specs/services/media-ingest.md).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/media/uploads", h.auth(h.create))
	mux.Handle("GET /api/v1/media/uploads/{id}", h.auth(h.get))
	mux.Handle("POST /api/v1/media/uploads/{id}/complete", h.auth(h.complete))
}

type actorHandler func(w http.ResponseWriter, r *http.Request, actor application.Actor)

// auth requires a valid access token and marks responses private.
func (h *Handler) auth(next actorHandler) http.Handler {
	return h.verifier.Required(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := authn.FromContext(r.Context())
		id, err := uuid.Parse(p.UserID)
		if !ok || err != nil {
			httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		next(w, r, application.Actor{ID: id, Artist: p.HasRole(authn.RoleArtist), Admin: p.HasRole(authn.RoleAdmin)})
	}))
}

type signedURLView struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type uploadView struct {
	ID            string         `json:"id"`
	TrackID       string         `json:"trackId"`
	Status        domain.Status  `json:"status"`
	ContentType   string         `json:"contentType"`
	SizeBytes     int64          `json:"sizeBytes"`
	SHA256        string         `json:"sha256"`
	FailureReason *string        `json:"failureReason"`
	CreatedAt     time.Time      `json:"createdAt"`
	ExpiresAt     time.Time      `json:"expiresAt"`
	CompletedAt   *time.Time     `json:"completedAt"`
	Upload        *signedURLView `json:"upload,omitempty"`
}

func view(u domain.Upload) uploadView {
	v := uploadView{
		ID: u.ID.String(), TrackID: u.TrackID.String(), Status: u.Status, ContentType: u.ContentType,
		SizeBytes: u.SizeBytes, SHA256: hex.EncodeToString(u.SHA256), CreatedAt: u.CreatedAt.UTC(), ExpiresAt: u.ExpiresAt.UTC(),
		CompletedAt: u.CompletedAt,
	}
	if u.FailureReason != "" {
		v.FailureReason = &u.FailureReason
	}
	return v
}

type createRequest struct {
	TrackID     string `json:"trackId"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
}

// create opens a session and returns the presigned PUT the client must use.
func (h *Handler) create(w http.ResponseWriter, r *http.Request, actor application.Actor) {
	var req createRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	trackID, _ := uuid.Parse(req.TrackID) // uuid.Nil fails validation
	c, err := h.app.Create(r.Context(), actor, domain.Request{TrackID: trackID, ContentType: req.ContentType, SizeBytes: req.SizeBytes, SHA256Hex: req.SHA256})
	if err != nil {
		h.error(w, r, err)
		return
	}
	v := view(c.Upload)
	v.Upload = &signedURLView{Method: c.URL.Method, URL: c.URL.URL, Headers: c.URL.Headers, ExpiresAt: c.URL.ExpiresAt.UTC()}
	w.Header().Set("Location", "/api/v1/media/uploads/"+c.Upload.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, v)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, actor application.Actor) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	u, err := h.app.Get(r.Context(), actor, id)
	if err != nil {
		h.error(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, view(u))
}

// complete verifies the uploaded object; a rejected upload answers 422 with
// the reason (the session is FAILED and a new one is needed).
func (h *Handler) complete(w http.ResponseWriter, r *http.Request, actor application.Actor) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	u, err := h.app.Complete(r.Context(), actor, id)
	if err != nil {
		h.error(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, view(u))
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter id must be a UUID", nil)
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) error(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ve *domain.ValidationError
		re *domain.RejectedError
	)
	switch {
	case errors.As(err, &ve):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": ve.Fields})
	case errors.As(err, &re):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, "UPLOAD_REJECTED", "The uploaded file failed verification", map[string]any{"reason": re.Reason})
	case errors.Is(err, domain.ErrForbidden):
		httpserver.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only artists and admins can upload media", nil)
	case errors.Is(err, domain.ErrTrackNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "TRACK_NOT_FOUND", "Track not found", nil)
	case errors.Is(err, domain.ErrNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "UPLOAD_NOT_FOUND", "Upload not found", nil)
	case errors.Is(err, domain.ErrNotUploaded):
		httpserver.WriteError(w, r, http.StatusConflict, "UPLOAD_INCOMPLETE", "The file has not been uploaded yet", nil)
	default:
		h.log.ErrorContext(r.Context(), "media ingest request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Media ingest is temporarily unavailable", nil)
	}
}
