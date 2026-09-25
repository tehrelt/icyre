// Package http serves the page-oriented API of the Web BFF (/api/v1/pages/*).
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/bff/internal/application"
	"github.com/tehrelt/icyre/services/bff/internal/ports"
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
	mux.Handle("GET /api/v1/pages/home", withUser(h.home))
	mux.Handle("GET /api/v1/pages/albums/{id}", withUser(h.album))
}

// withUser forwards the caller's bearer token to upstream calls. The BFF
// does not validate it: each upstream service verifies it on its own.
func withUser(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && token != "" {
			r = r.WithContext(ports.WithUserToken(r.Context(), token))
		}
		next(w, r)
	})
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	page, err := h.pages.Home(r.Context())
	if err != nil {
		h.fail(w, r, err, "")
		return
	}
	writePage(w, r, page)
}

func (h *Handler) album(w http.ResponseWriter, r *http.Request) {
	page, err := h.pages.Album(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err, "ALBUM_NOT_FOUND")
		return
	}
	writePage(w, r, page)
}

// writePage sends a page. Anonymous pages may be reused for a short while;
// a signed-in listener's page carries their own state (liked tracks), so the
// browser must revalidate it — otherwise a like would vanish on reload.
func writePage(w http.ResponseWriter, r *http.Request, page any) {
	w.Header().Set("Vary", "Authorization")
	if ports.UserToken(r.Context()) != "" {
		w.Header().Set("Cache-Control", "private, no-cache")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=30")
	}
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
