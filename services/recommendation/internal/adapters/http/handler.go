// Package http is the Recommendation API (specs/services/recommendation.md).
package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/recommendation/internal/application"
)

// Limits of the list endpoints and the sizes of the home sections.
const (
	defaultLimit = 20
	maxLimit     = 50
	homeTracks   = 20
	homeArtists  = 10
)

// Recommendations is the use case the handler serves.
type Recommendations interface {
	For(ctx context.Context, user uuid.UUID) (application.Result, error)
}

// Handler serves the API.
type Handler struct {
	app      Recommendations
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Recommendations, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes. Authentication is optional: guests (and
// users without a personal set) get the popular set.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/recommendations/home", h.verifier.Optional(http.HandlerFunc(h.home)))
	mux.Handle("GET /api/v1/recommendations/tracks", h.verifier.Optional(http.HandlerFunc(h.list(func(r application.Result) []recommendation.Item { return r.Tracks }))))
	mux.Handle("GET /api/v1/recommendations/artists", h.verifier.Optional(http.HandlerFunc(h.list(func(r application.Result) []recommendation.Item { return r.Artists }))))
}

type meta struct {
	Source      string     `json:"source"`
	Algorithm   string     `json:"algorithm,omitempty"`
	GeneratedAt *time.Time `json:"generatedAt,omitempty"`
}

type homeResponse struct {
	meta
	Tracks  []recommendation.Item `json:"tracks"`
	Artists []recommendation.Item `json:"artists"`
}

type listResponse struct {
	meta
	Data []recommendation.Item `json:"data"`
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	res, ok := h.result(w, r)
	if !ok {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, homeResponse{meta: metaOf(res), Tracks: head(res.Tracks, homeTracks), Artists: head(res.Artists, homeArtists)})
}

func (h *Handler) list(pick func(application.Result) []recommendation.Item) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultLimit
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > maxLimit {
				httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request",
					map[string]any{"fields": map[string]string{"limit": "must be an integer within 1..50"}})
				return
			}
			limit = n
		}
		res, ok := h.result(w, r)
		if !ok {
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, listResponse{meta: metaOf(res), Data: head(pick(res), limit)})
	}
}

// result reads the caller's set; responses are private either way (a guest
// and a user see different sets under the same URL).
func (h *Handler) result(w http.ResponseWriter, r *http.Request) (application.Result, bool) {
	user := uuid.Nil
	if p, ok := authn.FromContext(r.Context()); ok {
		if id, err := uuid.Parse(p.UserID); err == nil {
			user = id
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Vary", "Authorization")
	res, err := h.app.For(r.Context(), user)
	if err != nil {
		h.log.ErrorContext(r.Context(), "read recommendations", "error", err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, "RECOMMENDATIONS_UNAVAILABLE", "Recommendations are temporarily unavailable", nil)
		return application.Result{}, false
	}
	return res, true
}

func metaOf(r application.Result) meta {
	m := meta{Source: r.Source, Algorithm: r.Algorithm}
	if !r.GeneratedAt.IsZero() {
		t := r.GeneratedAt
		m.GeneratedAt = &t
	}
	return m
}

func head(items []recommendation.Item, n int) []recommendation.Item {
	if len(items) > n {
		return items[:n]
	}
	return items
}
