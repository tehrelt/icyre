// Package http serves the listener's history under /api/v1/me/history.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/history/internal/application"
	"github.com/tehrelt/icyre/services/history/internal/domain"
)

// History is the application layer.
type History interface {
	Tracks(ctx context.Context, userID uuid.UUID, after *domain.Cursor, limit int) (application.Page, error)
	RecentSources(ctx context.Context, userID uuid.UUID, limit int) ([]domain.RecentSource, error)
}

// Handler serves the history API.
type Handler struct {
	app      History
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app History, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/me/history/tracks", h.auth(h.tracks))
	mux.Handle("GET /api/v1/me/history/sources", h.auth(h.sources))
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
		w.Header().Set("Cache-Control", "private, no-store")
		next(w, r, user)
	}))
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]any) {
	httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
}

func limitParam(r *http.Request, maxLimit int, fields map[string]any) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxLimit {
		fields["limit"] = "must be between 1 and " + strconv.Itoa(maxLimit)
	}
	return n
}

type listenView struct {
	TrackID    string    `json:"trackId"`
	Source     *string   `json:"source"`
	ListenedMs int64     `json:"listenedMs"`
	PlayedAt   time.Time `json:"playedAt"`
}

type pagination struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

func (h *Handler) tracks(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	fields := map[string]any{}
	limit := limitParam(r, application.MaxLimit, fields)
	var after *domain.Cursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		c, err := domain.DecodeCursor(raw)
		if err != nil {
			fields["cursor"] = "is malformed"
		}
		after = &c
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	page, err := h.app.Tracks(r.Context(), user, after, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := struct {
		Data       []listenView `json:"data"`
		Pagination pagination   `json:"pagination"`
	}{Data: make([]listenView, len(page.Listens))}
	for i, l := range page.Listens {
		v := listenView{TrackID: l.TrackID.String(), ListenedMs: l.ListenedMs, PlayedAt: l.PlayedAt.UTC()}
		if l.Source != "" {
			s := l.Source
			v.Source = &s
		}
		out.Data[i] = v
	}
	if page.Next != nil {
		c := page.Next.Encode()
		out.Pagination = pagination{NextCursor: &c, HasMore: true}
	}
	httpserver.WriteJSON(w, http.StatusOK, out)
}

type sourceView struct {
	Source   string    `json:"source"`
	PlayedAt time.Time `json:"playedAt"`
}

func (h *Handler) sources(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	fields := map[string]any{}
	limit := limitParam(r, 50, fields)
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	list, err := h.app.RecentSources(r.Context(), user, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]sourceView, len(list))
	for i, s := range list {
		out[i] = sourceView{Source: s.Source, PlayedAt: s.PlayedAt.UTC()}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "history request failed", "error", err)
	httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "History is temporarily unavailable", nil)
}
