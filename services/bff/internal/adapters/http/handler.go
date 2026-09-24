// Package http serves the page-oriented API of the Web BFF (/api/v1/pages/*).
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/bff/internal/application"
	"github.com/tehrelt/icyre/services/bff/internal/views"
)

// Pages is what the handlers need from the application layer.
type Pages interface {
	Home(ctx context.Context) (views.HomePage, error)
	Album(ctx context.Context, id string) (views.AlbumPage, error)
}

// Handler serves page endpoints.
type Handler struct {
	pages Pages
	log   *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(pages Pages, log *slog.Logger) *Handler { return &Handler{pages: pages, log: log} }

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/pages/home", h.home)
	mux.HandleFunc("GET /api/v1/pages/albums/{id}", h.album)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	page, err := h.pages.Home(r.Context())
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	writePage(w, page)
}

func (h *Handler) album(w http.ResponseWriter, r *http.Request) {
	page, err := h.pages.Album(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err, "ALBUM_NOT_FOUND")
		return
	}
	writePage(w, page)
}

// writePage sends a page with a short private cache hint.
func writePage(w http.ResponseWriter, page any) {
	w.Header().Set("Cache-Control", "private, max-age=30")
	httpserver.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error, notFoundCode string) {
	switch {
	case errors.Is(err, application.ErrNotFound) && notFoundCode != "":
		httpserver.WriteError(w, r, http.StatusNotFound, notFoundCode, "Not found", nil)
	case errors.Is(err, application.ErrUpstream), errors.Is(err, context.DeadlineExceeded):
		h.log.WarnContext(r.Context(), "page aggregation failed", "error", err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "A service this page depends on is unavailable", nil)
	default:
		h.log.ErrorContext(r.Context(), "page request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, httpserver.CodeInternal, "Internal server error", nil)
	}
}
