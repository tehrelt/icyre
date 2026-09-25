// Package http serves POST /api/v1/playback/events.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

// Reporter is the application layer.
type Reporter interface {
	Report(ctx context.Context, e domain.Event) error
}

// Handler serves the playback API.
type Handler struct {
	app      Reporter
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Reporter, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/playback/events", h.verifier.Required(http.HandlerFunc(h.report)))
}

type eventRequest struct {
	Type       string `json:"type"`
	PlaybackID string `json:"playbackId"`
	TrackID    string `json:"trackId"`
	Source     string `json:"source"`
	DurationMs int64  `json:"durationMs"`
	ListenedMs int64  `json:"listenedMs"`
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	p, ok := authn.FromContext(r.Context())
	user, err := uuid.Parse(p.UserID)
	if !ok || err != nil {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req eventRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	// Unparsable IDs become uuid.Nil and are reported by validation.
	playbackID, _ := uuid.Parse(req.PlaybackID)
	trackID, _ := uuid.Parse(req.TrackID)
	err = h.app.Report(r.Context(), domain.Event{
		Kind: domain.Kind(req.Type), PlaybackID: playbackID, UserID: user, TrackID: trackID,
		Source: req.Source, DurationMs: req.DurationMs, ListenedMs: req.ListenedMs,
	})
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := map[string]any{}
		for k, v := range ve.Fields {
			fields[k] = v
		}
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
	case err != nil:
		h.log.ErrorContext(r.Context(), "publish playback event failed", "error", err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Playback telemetry is temporarily unavailable", nil)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}
