// Package http serves profiles: public GET /users/{id}, owner /users/me.
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
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

// Profiles is what the handlers need from the application layer.
type Profiles interface {
	Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	Update(ctx context.Context, userID uuid.UUID, c domain.Changes) (domain.Profile, error)
}

// Handler serves profile routes.
type Handler struct {
	app      Profiles
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Profiles, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes. "me" is matched before {id}.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/users/me", h.verifier.Required(http.HandlerFunc(h.me)))
	mux.Handle("PATCH /api/v1/users/me", h.verifier.Required(http.HandlerFunc(h.updateMe)))
	mux.HandleFunc("GET /api/v1/users/{id}", h.get)
}

// publicProfile is visible to anyone.
type publicProfile struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"` // media delivery arrives with EPIC-018
	Bio         string  `json:"bio"`
	Country     string  `json:"country"`
}

// ownProfile adds private settings.
type ownProfile struct {
	publicProfile
	Language  string    `json:"language"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type updateRequest struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"displayName"`
	Bio         *string `json:"bio"`
	Country     *string `json:"country"`
	Language    *string `json:"language"`
}

func toPublic(p domain.Profile) publicProfile {
	return publicProfile{ID: p.UserID.String(), Username: p.Username, DisplayName: p.DisplayName, Bio: p.Bio, Country: p.Country}
}

func toOwn(p domain.Profile) ownProfile {
	return ownProfile{publicProfile: toPublic(p), Language: p.Language, CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC()}
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter id must be a UUID", nil)
		return
	}
	p, err := h.app.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toPublic(p))
}

func principalID(r *http.Request) (uuid.UUID, bool) {
	p, ok := authn.FromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(p.UserID)
	return id, err == nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	p, err := h.app.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	httpserver.WriteJSON(w, http.StatusOK, toOwn(p))
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	id, ok := principalID(r)
	if !ok {
		httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
		return
	}
	var req updateRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	p, err := h.app.Update(r.Context(), id, domain.Changes(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toOwn(p))
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := map[string]any{}
		for k, v := range ve.Fields {
			fields[k] = v
		}
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
	case errors.Is(err, domain.ErrUsernameTaken):
		httpserver.WriteError(w, r, http.StatusConflict, "USERNAME_TAKEN", "This username is taken", nil)
	case errors.Is(err, domain.ErrProfileNotFound):
		// Right after sign-up the profile may still be on its way from auth.events.
		httpserver.WriteError(w, r, http.StatusNotFound, "PROFILE_NOT_FOUND", "Profile not found", nil)
	default:
		h.log.ErrorContext(r.Context(), "profile request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, httpserver.CodeInternal, "Internal server error", nil)
	}
}
