// Package http serves POST /api/v1/stream/authorize.
package http

import (
	"context"
	"errors"
	"net/http"
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

// Handler serves the stream API.
type Handler struct {
	app      Authorizer
	verifier *authn.Verifier
}

// NewHandler returns a Handler.
func NewHandler(app Authorizer, verifier *authn.Verifier) *Handler {
	return &Handler{app: app, verifier: verifier}
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/stream/authorize", h.verifier.Required(http.HandlerFunc(h.authorize)))
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
