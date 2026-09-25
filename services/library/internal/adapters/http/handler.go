// Package http serves the listener's library under /api/v1/me/library.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/library/internal/application"
	"github.com/tehrelt/icyre/services/library/internal/domain"
)

// Library is the application layer.
type Library interface {
	Save(ctx context.Context, userID uuid.UUID, kind domain.Kind, id uuid.UUID) (domain.Item, error)
	Remove(ctx context.Context, userID uuid.UUID, kind domain.Kind, id uuid.UUID) error
	List(ctx context.Context, userID uuid.UUID, kind domain.Kind, after *domain.Cursor, limit int) (application.Page, error)
	Contains(ctx context.Context, userID uuid.UUID, kind domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error)
	Counts(ctx context.Context, userID uuid.UUID) (domain.Counts, error)
}

// Handler serves the library API.
type Handler struct {
	app      Library
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Library, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes (specs/services/library.md, plus list/contains
// for albums and a summary for the sidebar counters).
func (h *Handler) Register(mux *http.ServeMux) {
	for _, k := range []domain.Kind{domain.KindTrack, domain.KindAlbum} {
		base := "/api/v1/me/library/" + string(k) + "s"
		mux.Handle("PUT "+base+"/{id}", h.auth(h.save(k)))
		mux.Handle("DELETE "+base+"/{id}", h.auth(h.remove(k)))
		mux.Handle("GET "+base, h.auth(h.list(k)))
		mux.Handle("GET "+base+"/contains", h.auth(h.contains(k)))
	}
	mux.Handle("GET /api/v1/me/library/summary", h.auth(h.summary))
}

type userHandler func(w http.ResponseWriter, r *http.Request, user uuid.UUID)

// auth requires a valid access token and marks responses private.
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

func notFoundCode(k domain.Kind) (string, string) {
	if k == domain.KindAlbum {
		return "ALBUM_NOT_FOUND", "Album not found"
	}
	return "TRACK_NOT_FOUND", "Track not found"
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter id must be a UUID", nil)
		return uuid.Nil, false
	}
	return id, true
}

// save is an idempotent PUT: 204 whether or not the item was already saved.
func (h *Handler) save(k domain.Kind) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if _, err := h.app.Save(r.Context(), user, k, id); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				code, msg := notFoundCode(k)
				httpserver.WriteError(w, r, http.StatusNotFound, code, msg, nil)
				return
			}
			h.fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// remove is an idempotent DELETE.
func (h *Handler) remove(k domain.Kind) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := h.app.Remove(r.Context(), user, k, id); err != nil {
			h.fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type itemView struct {
	TrackID string    `json:"trackId,omitempty"`
	AlbumID string    `json:"albumId,omitempty"`
	SavedAt time.Time `json:"savedAt"`
}

type pagination struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type listResponse struct {
	Data       []itemView `json:"data"`
	Pagination pagination `json:"pagination"`
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]any) {
	httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
}

func (h *Handler) list(k domain.Kind) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
		q := r.URL.Query()
		fields := map[string]any{}
		limit := domain.DefaultLimit
		if raw := q.Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > domain.MaxLimit {
				fields["limit"] = "must be between 1 and 100"
			}
			limit = n
		}
		var after *domain.Cursor
		if raw := q.Get("cursor"); raw != "" {
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
		page, err := h.app.List(r.Context(), user, k, after, limit)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		res := listResponse{Data: make([]itemView, len(page.Items))}
		for i, it := range page.Items {
			v := itemView{SavedAt: it.SavedAt.UTC()}
			if k == domain.KindAlbum {
				v.AlbumID = it.EntityID.String()
			} else {
				v.TrackID = it.EntityID.String()
			}
			res.Data[i] = v
		}
		if page.Next != nil {
			c := page.Next.Encode()
			res.Pagination = pagination{NextCursor: &c, HasMore: true}
		}
		httpserver.WriteJSON(w, http.StatusOK, res)
	}
}

// contains answers "which of these are saved?" for up to 100 IDs.
func (h *Handler) contains(k domain.Kind) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
		raw := strings.Split(r.URL.Query().Get("ids"), ",")
		ids := make([]uuid.UUID, 0, len(raw))
		for _, s := range raw {
			if s = strings.TrimSpace(s); s == "" {
				continue
			}
			id, err := uuid.Parse(s)
			if err != nil {
				invalid(w, r, map[string]any{"ids": "must be comma-separated UUIDs"})
				return
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 || len(ids) > domain.MaxContains {
			invalid(w, r, map[string]any{"ids": "must list 1 to 100 IDs"})
			return
		}
		saved, err := h.app.Contains(r.Context(), user, k, ids)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		out := make([]string, len(saved))
		for i, id := range saved {
			out[i] = id.String()
		}
		httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
	}
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	c, err := h.app.Counts(r.Context(), user)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]int{"tracks": c.Tracks, "albums": c.Albums})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "library request failed", "error", err)
	httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Library is temporarily unavailable", nil)
}
