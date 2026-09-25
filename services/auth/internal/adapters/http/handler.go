// Package http exposes Auth over REST (/api/v1/auth/*). The refresh token
// travels only in an HttpOnly cookie scoped to the auth path.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/auth/internal/application"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

// RefreshCookie is the name of the refresh-token cookie.
const RefreshCookie = "icyre_refresh"

const cookiePath = "/api/v1/auth"

// Auth is the subset of application.Service used by handlers.
type Auth interface {
	Register(ctx context.Context, email, password string, c application.Client) (application.Result, error)
	Login(ctx context.Context, email, password string, c application.Client) (application.Result, error)
	Refresh(ctx context.Context, token string) (application.Result, error)
	Logout(ctx context.Context, token string) error
	Sessions(ctx context.Context, userID uuid.UUID) ([]domain.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
}

// Options configure cookies.
type Options struct {
	// SecureCookies must be true behind HTTPS (everywhere but local HTTP).
	SecureCookies bool
}

// Handler serves the auth routes.
type Handler struct {
	app      Auth
	verifier *authn.Verifier
	jwks     authn.JWKS
	opts     Options
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Auth, verifier *authn.Verifier, jwks authn.JWKS, opts Options, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, jwks: jwks, opts: opts, log: log}
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.Handle("GET /api/v1/auth/sessions", h.verifier.Required(http.HandlerFunc(h.sessions)))
	mux.Handle("DELETE /api/v1/auth/sessions/{id}", h.verifier.Required(http.HandlerFunc(h.revokeSession)))
	mux.HandleFunc("GET /api/v1/auth/.well-known/jwks.json", h.keys)
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

type tokenResponse struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresIn   int       `json:"expiresIn"`
	ExpiresAt   time.Time `json:"expiresAt"`
	User        userDTO   `json:"user"`
}

type sessionDTO struct {
	ID         string    `json:"id"`
	UserAgent  string    `json:"userAgent"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Current    bool      `json:"current"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	res, err := h.app.Register(r.Context(), req.Email, req.Password, client(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeTokens(w, http.StatusCreated, res)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, err.Error(), nil)
		return
	}
	res, err := h.app.Login(r.Context(), req.Email, req.Password, client(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.writeTokens(w, http.StatusOK, res)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	res, err := h.app.Refresh(r.Context(), refreshToken(r))
	if err != nil {
		h.clearCookie(w)
		h.fail(w, r, err)
		return
	}
	h.writeTokens(w, http.StatusOK, res)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Logout(r.Context(), refreshToken(r)); err != nil {
		h.fail(w, r, err)
		return
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	p, _ := authn.FromContext(r.Context())
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	list, err := h.app.Sessions(r.Context(), uid)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]sessionDTO, 0, len(list))
	for _, s := range list {
		out = append(out, sessionDTO{ID: s.ID.String(), UserAgent: s.UserAgent, CreatedAt: s.CreatedAt, LastUsedAt: s.LastUsedAt, ExpiresAt: s.ExpiresAt, Current: s.ID.String() == p.SessionID})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	p, _ := authn.FromContext(r.Context())
	uid, err1 := uuid.Parse(p.UserID)
	sid, err2 := uuid.Parse(r.PathValue("id"))
	if err1 != nil || err2 != nil {
		httpserver.WriteError(w, r, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found", nil)
		return
	}
	if err := h.app.RevokeSession(r.Context(), uid, sid); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) keys(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpserver.WriteJSON(w, http.StatusOK, h.jwks)
}

func (h *Handler) writeTokens(w http.ResponseWriter, status int, res application.Result) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookie,
		Value:    res.RefreshToken,
		Path:     cookiePath,
		Expires:  res.RefreshUntil,
		MaxAge:   int(time.Until(res.RefreshUntil).Seconds()),
		HttpOnly: true,
		Secure:   h.opts.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, status, tokenResponse{
		AccessToken: res.Access.Token,
		TokenType:   "Bearer",
		ExpiresIn:   int(time.Until(res.Access.ExpiresAt).Seconds()),
		ExpiresAt:   res.Access.ExpiresAt.UTC(),
		User:        userDTO{ID: res.Account.ID.String(), Email: res.Account.Email, Roles: res.Account.Roles},
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: RefreshCookie, Value: "", Path: cookiePath, MaxAge: -1, HttpOnly: true, Secure: h.opts.SecureCookies, SameSite: http.SameSiteStrictMode})
}

// refreshToken reads the cookie; native clients may send {"refreshToken": ...}.
func refreshToken(r *http.Request) string {
	if c, err := r.Cookie(RefreshCookie); err == nil && c.Value != "" {
		return c.Value
	}
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if r.ContentLength > 0 && httpserver.DecodeJSON(nil, r, &body) == nil {
		return body.RefreshToken
	}
	return ""
}

func client(r *http.Request) application.Client {
	return application.Client{UserAgent: r.UserAgent()}
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
	case errors.Is(err, domain.ErrEmailTaken):
		httpserver.WriteError(w, r, http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists", nil)
	case errors.Is(err, domain.ErrInvalidCredentials):
		httpserver.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is incorrect", nil)
	case errors.Is(err, domain.ErrTooManyAttempts):
		w.Header().Set("Retry-After", strconv.Itoa(15*60))
		httpserver.WriteError(w, r, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Too many sign-in attempts, try again later", nil)
	case errors.Is(err, domain.ErrInvalidRefresh), errors.Is(err, domain.ErrSessionInactive), errors.Is(err, domain.ErrRefreshReuse), errors.Is(err, domain.ErrConcurrentUpdate):
		httpserver.WriteError(w, r, http.StatusUnauthorized, "SESSION_EXPIRED", "Session expired, sign in again", nil)
	case errors.Is(err, domain.ErrSessionNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found", nil)
	default:
		h.log.ErrorContext(r.Context(), "auth request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, httpserver.CodeInternal, "Internal server error", nil)
	}
}
