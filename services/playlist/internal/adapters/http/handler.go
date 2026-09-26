// Package http serves the playlist API.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

// Playlists is the application layer.
type Playlists interface {
	Create(ctx context.Context, owner uuid.UUID, title string) (domain.Playlist, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Playlist, []domain.Track, error)
	Mine(ctx context.Context, owner uuid.UUID) ([]domain.Playlist, error)
	AddTrack(ctx context.Context, caller, id, trackID uuid.UUID) error
	RemoveTrack(ctx context.Context, caller, id, trackID uuid.UUID) error
	Rename(ctx context.Context, caller, id uuid.UUID, title string) (domain.Playlist, error)
	Delete(ctx context.Context, caller, id uuid.UUID) error
	Reorder(ctx context.Context, caller, id uuid.UUID, order []uuid.UUID) error
}

// Handler serves the playlist API.
type Handler struct {
	app      Playlists
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Playlists, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// MaxOrder bounds the track list of one reorder request.
const MaxOrder = 10_000

// Register mounts the routes of specs/services/playlist.md.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/playlists", h.auth(h.create))
	mux.Handle("GET /api/v1/me/playlists", h.auth(h.mine))
	mux.HandleFunc("GET /api/v1/playlists/{id}", h.get)
	mux.Handle("PATCH /api/v1/playlists/{id}", h.auth(h.rename))
	mux.Handle("DELETE /api/v1/playlists/{id}", h.auth(h.delete))
	mux.Handle("PATCH /api/v1/playlists/{id}/tracks/order", h.auth(h.reorder))
	mux.Handle("POST /api/v1/playlists/{id}/tracks", h.auth(h.addTrack))
	mux.Handle("DELETE /api/v1/playlists/{id}/tracks/{trackId}", h.auth(h.removeTrack))
}

type userHandler func(w http.ResponseWriter, r *http.Request, user uuid.UUID)

func (h *Handler) auth(next userHandler) http.Handler {
	return h.verifier.Required(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := authn.FromContext(r.Context())
		user, err := uuid.Parse(p.UserID)
		if !ok || err != nil {
			httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
			return
		}
		next(w, r, user)
	}))
}

type playlistView struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	OwnerID    string    `json:"ownerId"`
	TrackCount int       `json:"trackCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type trackView struct {
	TrackID  string    `json:"trackId"`
	Position int       `json:"position"`
	AddedAt  time.Time `json:"addedAt"`
}

func view(p domain.Playlist) playlistView {
	return playlistView{ID: p.ID.String(), Title: p.Title, OwnerID: p.OwnerID.String(), TrackCount: p.TrackCount, CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC()}
}

func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter "+name+" must be a UUID", nil)
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	var req struct {
		Title string `json:"title"`
	}
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	p, err := h.app.Create(r.Context(), user, req.Title)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/playlists/"+p.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, view(p))
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	list, err := h.app.Mine(r.Context(), user)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]playlistView, len(list))
	for i, p := range list {
		out[i] = view(p)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	p, tracks, err := h.app.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := struct {
		playlistView
		Tracks []trackView `json:"tracks"`
	}{playlistView: view(p), Tracks: make([]trackView, len(tracks))}
	for i, t := range tracks {
		out.Tracks[i] = trackView{TrackID: t.TrackID.String(), Position: t.Position, AddedAt: t.AddedAt.UTC()}
	}
	httpserver.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) addTrack(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		TrackID string `json:"trackId"`
	}
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	trackID, err := uuid.Parse(req.TrackID)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": map[string]any{"trackId": "must be a track ID"}})
		return
	}
	if err := h.app.AddTrack(r.Context(), user, id, trackID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeTrack(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	trackID, ok := pathUUID(w, r, "trackId")
	if !ok {
		return
	}
	if err := h.app.RemoveTrack(r.Context(), user, id, trackID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	p, err := h.app.Rename(r.Context(), user, id, req.Title)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, view(p))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.app.Delete(r.Context(), user, id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		TrackIDs []string `json:"trackIds"`
	}
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	if req.TrackIDs == nil || len(req.TrackIDs) > MaxOrder {
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": map[string]any{"trackIds": "must list the playlist's track IDs"}})
		return
	}
	order := make([]uuid.UUID, len(req.TrackIDs))
	for i, s := range req.TrackIDs {
		tid, err := uuid.Parse(s)
		if err != nil {
			httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": map[string]any{"trackIds": "must be track IDs"}})
			return
		}
		order[i] = tid
	}
	if err := h.app.Reorder(r.Context(), user, id, order); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidTitle):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": map[string]any{"title": "must be 1–100 characters"}})
	case errors.Is(err, domain.ErrNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "PLAYLIST_NOT_FOUND", "Playlist not found", nil)
	case errors.Is(err, domain.ErrTrackNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "TRACK_NOT_FOUND", "Track not found", nil)
	case errors.Is(err, domain.ErrOrderMismatch):
		httpserver.WriteError(w, r, http.StatusConflict, "PLAYLIST_ORDER_MISMATCH", "The order must list every track of the playlist exactly once", nil)
	case errors.Is(err, domain.ErrForbidden):
		httpserver.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only the owner can change this playlist", nil)
	default:
		h.log.ErrorContext(r.Context(), "playlist request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Playlists are temporarily unavailable", nil)
	}
}
