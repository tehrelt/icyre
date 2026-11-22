// Package http serves upload sessions under /api/v1/media/uploads.
package http

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	ListModerationUploads(ctx context.Context, trackID *uuid.UUID, status *domain.Status, afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]domain.Upload, error)
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

// RegisterInternal mounts internal moderation routes protected by X-Internal-Token.
func (h *Handler) RegisterInternal(mux *http.ServeMux, internalToken string) {
	guard := authn.RequireInternalToken(internalToken)
	mux.Handle("GET /internal/v1/moderation/uploads", guard(http.HandlerFunc(h.listModerationUploads)))
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

type moderationUploadView struct {
	ID            string        `json:"id"`
	TrackID       string        `json:"trackId"`
	Status        domain.Status `json:"status"`
	FailureReason *string       `json:"failureReason"`
	CreatedAt     time.Time     `json:"createdAt"`
	ExpiresAt     time.Time     `json:"expiresAt"`
	CompletedAt   *time.Time    `json:"completedAt"`
}

type paginationView struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type moderationUploadsResponse struct {
	Data       []moderationUploadView `json:"data"`
	Pagination paginationView         `json:"pagination"`
}

func toModerationUploadView(u domain.Upload) moderationUploadView {
	v := moderationUploadView{
		ID:        u.ID.String(),
		TrackID:   u.TrackID.String(),
		Status:    u.Status,
		CreatedAt: u.CreatedAt.UTC(),
		ExpiresAt: u.ExpiresAt.UTC(),
	}
	if u.FailureReason != "" {
		r := u.FailureReason
		v.FailureReason = &r
	}
	if u.CompletedAt != nil {
		t := u.CompletedAt.UTC()
		v.CompletedAt = &t
	}
	return v
}

var errInvalidCursor = errors.New("cursor is malformed")

func (h *Handler) listModerationUploads(w http.ResponseWriter, r *http.Request) {
	trackID, status, limit, afterCreatedAt, afterID, err := parseModerationUploadsQuery(r)
	if err != nil {
		if errors.Is(err, errInvalidCursor) {
			httpserver.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "cursor is malformed", nil)
			return
		}
		h.error(w, r, err)
		return
	}
	uploads, err := h.app.ListModerationUploads(r.Context(), trackID, status, afterCreatedAt, afterID, limit+1)
	if err != nil {
		h.error(w, r, err)
		return
	}
	p := paginationView{HasMore: len(uploads) > limit}
	if p.HasMore {
		uploads = uploads[:limit]
		last := uploads[limit-1]
		c := encodeUploadCursor(last.CreatedAt, last.ID)
		p.NextCursor = &c
	}
	data := make([]moderationUploadView, len(uploads))
	for i, u := range uploads {
		data[i] = toModerationUploadView(u)
	}
	httpserver.WriteJSON(w, http.StatusOK, moderationUploadsResponse{Data: data, Pagination: p})
}

func parseModerationUploadsQuery(r *http.Request) (*uuid.UUID, *domain.Status, int, *time.Time, *uuid.UUID, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	var trackID *uuid.UUID
	if raw := q.Get("trackId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fields["trackId"] = "must be a UUID"
		} else {
			trackID = &id
		}
	}
	var status *domain.Status
	if raw := q.Get("status"); raw != "" {
		s := domain.Status(strings.ToUpper(raw))
		if !s.Valid() {
			fields["status"] = "must be one of PENDING, COMPLETED, FAILED"
		} else {
			status = &s
		}
	}
	limit := application.DefaultModerationLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > application.MaxModerationLimit {
			fields["limit"] = "must be between 1 and " + strconv.Itoa(application.MaxModerationLimit)
		} else {
			limit = n
		}
	}
	if len(fields) > 0 {
		return nil, nil, 0, nil, nil, &domain.ValidationError{Fields: fields}
	}
	var (
		afterCreatedAt *time.Time
		afterID        *uuid.UUID
	)
	if raw := q.Get("cursor"); raw != "" {
		t, id, err := decodeUploadCursor(raw)
		if err != nil {
			return nil, nil, 0, nil, nil, errInvalidCursor
		}
		afterCreatedAt, afterID = &t, &id
	}
	return trackID, status, limit, afterCreatedAt, afterID, nil
}

type uploadCursorJSON struct {
	T string `json:"t"`
	I string `json:"i"`
}

func encodeUploadCursor(createdAt time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(uploadCursorJSON{T: createdAt.UTC().Format(time.RFC3339Nano), I: id.String()})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeUploadCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	var c uploadCursorJSON
	if err := json.Unmarshal(b, &c); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	t, err := time.Parse(time.RFC3339Nano, c.T)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(c.I)
	if err != nil {
		return time.Time{}, uuid.Nil, errors.New("bad cursor id")
	}
	return t, id, nil
}
