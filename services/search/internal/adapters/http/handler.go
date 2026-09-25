// Package http serves GET /api/v1/search and GET /api/v1/search/suggest.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/search/internal/application"
)

// Searcher is the application layer.
type Searcher interface {
	Search(ctx context.Context, q string, t application.Type, limit int) (application.Result, error)
	Suggest(ctx context.Context, q string, limit int) ([]application.Suggestion, error)
}

// Handler serves the search API.
type Handler struct {
	app Searcher
	log *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Searcher, log *slog.Logger) *Handler { return &Handler{app: app, log: log} }

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/search", h.search)
	mux.HandleFunc("GET /api/v1/search/suggest", h.suggest)
}

// Results are the same for every listener; a short shared cache absorbs
// repeated keystrokes and popular queries.
const cacheControl = "public, max-age=30"

func limitParam(r *http.Request, def int, fields map[string]any) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		fields["limit"] = "must be a positive integer"
	}
	return n
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	fields := map[string]any{}
	t, ok := application.ParseType(r.URL.Query().Get("type"))
	if !ok {
		fields["type"] = "must be one of all, tracks, artists, albums, playlists"
	}
	limit := limitParam(r, 20, fields)
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	res, err := h.app.Search(r.Context(), r.URL.Query().Get("q"), t, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", cacheControl)
	httpserver.WriteJSON(w, http.StatusOK, toResponse(res))
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	fields := map[string]any{}
	limit := limitParam(r, 8, fields)
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	q := r.URL.Query().Get("q")
	items, err := h.app.Suggest(r.Context(), q, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := suggestResponse{Query: q, Suggestions: make([]suggestionView, len(items))}
	for i, s := range items {
		out.Suggestions[i] = suggestionView{Kind: s.Kind, ID: s.ID, Text: s.Text, Subtitle: s.Subtitle}
	}
	w.Header().Set("Cache-Control", cacheControl)
	httpserver.WriteJSON(w, http.StatusOK, out)
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]any) {
	httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, application.ErrInvalidQuery) {
		invalid(w, r, map[string]any{"q": "must be 1–200 characters"})
		return
	}
	// The index is derived data: when it is unreachable, search is
	// temporarily unavailable; nothing else is affected.
	h.log.ErrorContext(r.Context(), "search failed", "error", err)
	httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Search is temporarily unavailable", nil)
}
