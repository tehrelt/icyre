// Package http serves playback telemetry and session/queue control endpoints.
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
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

// Playback is the application layer.
type Playback interface {
	Report(ctx context.Context, e domain.Event) error
	StartSession(ctx context.Context, userID uuid.UUID, req domain.StartSessionRequest) (domain.PlaybackState, error)
	GetSession(ctx context.Context, userID uuid.UUID) (domain.PlaybackState, error)
	SetQueue(ctx context.Context, userID uuid.UUID, req domain.SetQueueRequest) (domain.PlaybackState, error)
	UpdateState(ctx context.Context, userID uuid.UUID, patch domain.StatePatch) (domain.PlaybackState, error)
}

// Handler serves the playback API.
type Handler struct {
	app      Playback
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Playback, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/playback/events", h.verifier.Required(http.HandlerFunc(h.report)))
	mux.Handle("POST /api/v1/playback/sessions", h.verifier.Required(http.HandlerFunc(h.startSession)))
	mux.Handle("GET /api/v1/playback/session", h.verifier.Required(http.HandlerFunc(h.getSession)))
	mux.Handle("PUT /api/v1/playback/queue", h.verifier.Required(http.HandlerFunc(h.setQueue)))
	mux.Handle("PATCH /api/v1/playback/state", h.verifier.Required(http.HandlerFunc(h.updateState)))
}

type eventRequest struct {
	Type       string `json:"type"`
	PlaybackID string `json:"playbackId"`
	TrackID    string `json:"trackId"`
	Source     string `json:"source"`
	DurationMs int64  `json:"durationMs"`
	ListenedMs int64  `json:"listenedMs"`
}

type startSessionRequest struct {
	Tracks     []string `json:"tracks"`
	StartIndex int      `json:"startIndex"`
	Source     string   `json:"source"`
	DurationMs int64    `json:"durationMs"`
	Shuffle    bool     `json:"shuffle"`
	RepeatMode string   `json:"repeatMode"`
}

type setQueueRequest struct {
	Tracks     []string `json:"tracks"`
	StartIndex *int     `json:"startIndex"`
	Source     string   `json:"source"`
}

type updateStateRequest struct {
	IsPlaying  *bool   `json:"isPlaying"`
	PositionMs *int64  `json:"positionMs"`
	DurationMs *int64  `json:"durationMs"`
	Shuffle    *bool   `json:"shuffle"`
	RepeatMode *string `json:"repeatMode"`
	QueueIndex *int    `json:"queueIndex"`
	Action     *string `json:"action"`
}

type queueView struct {
	Tracks []string `json:"tracks"`
	Order  []int    `json:"order"`
	Cursor int      `json:"cursor"`
	Source string   `json:"source,omitempty"`
}

type sessionView struct {
	SessionID  string            `json:"sessionId"`
	TrackID    *string           `json:"trackId"`
	PlaybackID *string           `json:"playbackId"`
	Source     string            `json:"source,omitempty"`
	IsPlaying  bool              `json:"isPlaying"`
	PositionMs int64             `json:"positionMs"`
	DurationMs int64             `json:"durationMs"`
	Shuffle    bool              `json:"shuffle"`
	RepeatMode domain.RepeatMode `json:"repeatMode"`
	QueueIndex int               `json:"queueIndex"`
	Queue      queueView         `json:"queue"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

func toSessionView(st domain.PlaybackState) sessionView {
	tracks := make([]string, len(st.Queue.Tracks))
	for i, id := range st.Queue.Tracks {
		tracks[i] = id.String()
	}
	order := st.Queue.Order
	if order == nil {
		order = []int{}
	}
	var trackID, playbackID *string
	if st.Session.TrackID != uuid.Nil {
		s := st.Session.TrackID.String()
		trackID = &s
	}
	if st.Session.PlaybackID != uuid.Nil {
		s := st.Session.PlaybackID.String()
		playbackID = &s
	}
	return sessionView{
		SessionID:  st.Session.SessionID.String(),
		TrackID:    trackID,
		PlaybackID: playbackID,
		Source:     st.Session.Source,
		IsPlaying:  st.Session.IsPlaying,
		PositionMs: st.Session.PositionMs,
		DurationMs: st.Session.DurationMs,
		Shuffle:    st.Session.Shuffle,
		RepeatMode: st.Session.RepeatMode,
		QueueIndex: st.Session.QueueIndex,
		Queue: queueView{
			Tracks: tracks,
			Order:  order,
			Cursor: st.Queue.Cursor,
			Source: st.Queue.Source,
		},
		UpdatedAt: st.Session.UpdatedAt.UTC(),
	}
}

func principalID(r *http.Request) (uuid.UUID, bool) {
	p, ok := authn.FromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(p.UserID)
	return id, err == nil
}

func parseTrackIDs(raw []string) []uuid.UUID {
	if raw == nil {
		return nil
	}
	out := make([]uuid.UUID, len(raw))
	for i, s := range raw {
		id, _ := uuid.Parse(s) // uuid.Nil fails domain validation
		out[i] = id
	}
	return out
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	user, ok := principalID(r)
	if !ok {
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
	err := h.app.Report(r.Context(), domain.Event{
		Kind: domain.Kind(req.Type), PlaybackID: playbackID, UserID: user, TrackID: trackID,
		Source: req.Source, DurationMs: req.DurationMs, ListenedMs: req.ListenedMs,
	})
	if err != nil {
		h.fail(w, r, err, "publish playback event failed", "Playback telemetry is temporarily unavailable")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	user, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req startSessionRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	st, err := h.app.StartSession(r.Context(), user, domain.StartSessionRequest{
		Tracks:     parseTrackIDs(req.Tracks),
		StartIndex: req.StartIndex,
		Source:     req.Source,
		DurationMs: req.DurationMs,
		Shuffle:    req.Shuffle,
		RepeatMode: domain.RepeatMode(req.RepeatMode),
	})
	if err != nil {
		h.fail(w, r, err, "start playback session failed", "Playback service is temporarily unavailable")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusCreated, toSessionView(st))
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	user, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	st, err := h.app.GetSession(r.Context(), user)
	if err != nil {
		h.fail(w, r, err, "get playback session failed", "Playback service is temporarily unavailable")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusOK, toSessionView(st))
}

func (h *Handler) setQueue(w http.ResponseWriter, r *http.Request) {
	user, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req setQueueRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	tracks := parseTrackIDs(req.Tracks)
	if tracks == nil {
		tracks = []uuid.UUID{}
	}
	st, err := h.app.SetQueue(r.Context(), user, domain.SetQueueRequest{
		Tracks:     tracks,
		StartIndex: req.StartIndex,
		Source:     req.Source,
	})
	if err != nil {
		h.fail(w, r, err, "set playback queue failed", "Playback service is temporarily unavailable")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusOK, toSessionView(st))
}

func (h *Handler) updateState(w http.ResponseWriter, r *http.Request) {
	user, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req updateStateRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	var repeat *domain.RepeatMode
	if req.RepeatMode != nil {
		m := domain.RepeatMode(*req.RepeatMode)
		repeat = &m
	}
	var action *domain.Action
	if req.Action != nil {
		a := domain.Action(*req.Action)
		action = &a
	}
	st, err := h.app.UpdateState(r.Context(), user, domain.StatePatch{
		IsPlaying:  req.IsPlaying,
		PositionMs: req.PositionMs,
		DurationMs: req.DurationMs,
		Shuffle:    req.Shuffle,
		RepeatMode: repeat,
		QueueIndex: req.QueueIndex,
		Action:     action,
	})
	if err != nil {
		h.fail(w, r, err, "update playback state failed", "Playback service is temporarily unavailable")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpserver.WriteJSON(w, http.StatusOK, toSessionView(st))
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error, logMsg, unavailableMsg string) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := map[string]any{}
		for k, v := range ve.Fields {
			fields[k] = v
		}
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
	case errors.Is(err, domain.ErrSessionNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "SESSION_NOT_FOUND", "Playback session not found", nil)
	default:
		h.log.ErrorContext(r.Context(), logMsg, "error", err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, unavailableMsg, nil)
	}
}
