// Package http serves POST /api/v1/stream/authorize.
package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/stream-auth/internal/application"
	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

// Authorizer is the use case.
type Authorizer interface {
	Authorize(ctx context.Context, r application.Request) (domain.Grant, error)
}

// Proxy streams audio through the service instead of signing a URL
// (EPIC-038 experiment "audio through the backend vs object storage").
type Proxy interface {
	Choose(ctx context.Context, r application.Request) (media.Quality, error)
	Stream(ctx context.Context, trackID string, q media.Quality, w io.Writer) (int64, error)
}

// Handler serves the stream API.
type Handler struct {
	app      Authorizer
	verifier *authn.Verifier
	proxy    Proxy
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Authorizer, verifier *authn.Verifier) *Handler {
	return &Handler{app: app, verifier: verifier, log: slog.Default()}
}

// WithProxy enables GET /api/v1/stream/proxy/{trackId}. Off by default: in
// the target architecture audio never passes through the backend.
func (h *Handler) WithProxy(p Proxy, log *slog.Logger) *Handler {
	h.proxy, h.log = p, log
	return h
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/stream/authorize", h.verifier.Required(http.HandlerFunc(h.authorize)))
	if h.proxy != nil {
		mux.Handle("GET /api/v1/stream/proxy/{trackId}", h.verifier.Required(http.HandlerFunc(h.stream)))
	}
}

type authorizeRequest struct {
	TrackID string `json:"trackId"`
	// Quality in kbps as a string ("64", "128", "256"); optional.
	Quality string `json:"quality"`
}

type grantResponse struct {
	TrackID   string    `json:"trackId"`
	Quality   string    `json:"quality"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) {
	p, ok := authn.FromContext(r.Context())
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req authorizeRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	fields := map[string]any{}
	if _, err := uuid.Parse(req.TrackID); err != nil {
		fields["trackId"] = "must be a track ID"
	}
	var q media.Quality
	if req.Quality != "" {
		var err error
		if q, err = media.ParseQuality(req.Quality); err != nil {
			fields["quality"] = "must be one of 64, 128, 256"
		}
	}
	if len(fields) > 0 {
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
		return
	}

	g, err := h.app.Authorize(r.Context(), application.Request{UserID: p.UserID, SessionID: p.SessionID, TrackID: req.TrackID, Quality: q})
	if err != nil {
		fail(w, r, err)
		return
	}
	// The URL is a short-lived bearer credential: never cache the response.
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, http.StatusOK, grantResponse{TrackID: g.TrackID, Quality: g.Quality.String(), URL: g.URL, ExpiresAt: g.ExpiresAt.UTC()})
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	p, ok := authn.FromContext(r.Context())
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	trackID := r.PathValue("trackId")
	fields := map[string]any{}
	if _, err := uuid.Parse(trackID); err != nil {
		fields["trackId"] = "must be a track ID"
	}
	var q media.Quality
	if raw := r.URL.Query().Get("quality"); raw != "" {
		var err error
		if q, err = media.ParseQuality(raw); err != nil {
			fields["quality"] = "must be one of 64, 128, 256"
		}
	}
	if len(fields) > 0 {
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
		return
	}

	q, err := h.proxy.Choose(r.Context(), application.Request{UserID: p.UserID, SessionID: p.SessionID, TrackID: trackID, Quality: q})
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", media.AudioContentType)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Audio-Quality", strconv.Itoa(int(q)))
	// Headers are sent with the first byte: a failure mid-stream can only
	// cut the response short.
	if n, err := h.proxy.Stream(r.Context(), trackID, q, w); err != nil {
		if n == 0 {
			fail(w, r, err)
			return
		}
		h.log.WarnContext(r.Context(), "audio stream interrupted", "track_id", trackID, "bytes", n, "error", err)
	}
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrTrackNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "TRACK_NOT_FOUND", "Track not found", nil)
	case errors.Is(err, domain.ErrTrackBlocked):
		httpserver.WriteError(w, r, http.StatusForbidden, "TRACK_UNAVAILABLE", "This track is not available for playback", nil)
	case errors.Is(err, domain.ErrTrackNotReady), errors.Is(err, domain.ErrNoVariant):
		httpserver.WriteError(w, r, http.StatusConflict, "TRACK_NOT_READY", "This track is still being processed", nil)
	default:
		// Catalog or storage is down; details are in the audit log.
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Service is temporarily unavailable", nil)
	}
}
