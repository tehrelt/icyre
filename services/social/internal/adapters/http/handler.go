// Package http serves the social graph: follow/unfollow, followers, following
// and counters for users and artists.
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
	"github.com/tehrelt/icyre/services/social/internal/application"
	"github.com/tehrelt/icyre/services/social/internal/domain"
)

// Social is the application layer.
type Social interface {
	Follow(ctx context.Context, follower uuid.UUID, target domain.Target) (domain.Follow, error)
	Unfollow(ctx context.Context, follower uuid.UUID, target domain.Target) error
	Followers(ctx context.Context, target domain.Target, after *domain.Cursor, limit int) (application.Page, error)
	Following(ctx context.Context, follower uuid.UUID, typ domain.TargetType, after *domain.Cursor, limit int) (application.Page, error)
	Contains(ctx context.Context, follower uuid.UUID, typ domain.TargetType, ids []uuid.UUID) ([]uuid.UUID, error)
	Counts(ctx context.Context, subject domain.Target) (domain.Counts, error)
}

// Handler serves the social API.
type Handler struct {
	app      Social
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Social, verifier *authn.Verifier, log *slog.Logger) *Handler {
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts the routes (specs/services/social.md, plus artists,
// counters and a "do I follow these?" lookup for Follow buttons).
func (h *Handler) Register(mux *http.ServeMux) {
	for _, t := range []domain.TargetType{domain.TargetUser, domain.TargetArtist} {
		base := "/api/v1/" + string(t) + "s/{id}"
		mux.Handle("PUT "+base+"/follow", h.auth(h.follow(t)))
		mux.Handle("DELETE "+base+"/follow", h.auth(h.unfollow(t)))
		mux.HandleFunc("GET "+base+"/followers", h.followers(t))
		mux.HandleFunc("GET "+base+"/follow-counts", h.counts(t))
	}
	mux.HandleFunc("GET /api/v1/users/{id}/following", h.following)
	mux.Handle("GET /api/v1/me/following/contains", h.auth(h.contains))
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

func notFound(w http.ResponseWriter, r *http.Request, t domain.TargetType) {
	if t == domain.TargetArtist {
		httpserver.WriteError(w, r, http.StatusNotFound, "ARTIST_NOT_FOUND", "Artist not found", nil)
		return
	}
	httpserver.WriteError(w, r, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, httpserver.CodeBadRequest, "path parameter id must be a UUID", nil)
		return uuid.Nil, false
	}
	return id, true
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]any) {
	httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
}

// follow is an idempotent PUT: 204 whether or not the target was already followed.
func (h *Handler) follow(t domain.TargetType) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		_, err := h.app.Follow(r.Context(), user, domain.Target{Type: t, ID: id})
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, domain.ErrNotFound):
			notFound(w, r, t)
		case errors.Is(err, domain.ErrSelfFollow):
			httpserver.WriteError(w, r, http.StatusUnprocessableEntity, "CANNOT_FOLLOW_SELF", "You cannot follow yourself", nil)
		default:
			h.fail(w, r, err)
		}
	}
}

// unfollow is an idempotent DELETE.
func (h *Handler) unfollow(t domain.TargetType) userHandler {
	return func(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := h.app.Unfollow(r.Context(), user, domain.Target{Type: t, ID: id}); err != nil {
			h.fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type edgeView struct {
	UserID     string    `json:"userId,omitempty"`
	ArtistID   string    `json:"artistId,omitempty"`
	FollowedAt time.Time `json:"followedAt"`
}

type pagination struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type listResponse struct {
	Data       []edgeView `json:"data"`
	Pagination pagination `json:"pagination"`
}

// pageParams reads limit and cursor; ok is false once a 422 was written.
func pageParams(w http.ResponseWriter, r *http.Request, fields map[string]any) (after *domain.Cursor, limit int, ok bool) {
	q := r.URL.Query()
	limit = domain.DefaultLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > domain.MaxLimit {
			fields["limit"] = "must be between 1 and 100"
		}
		limit = n
	}
	if raw := q.Get("cursor"); raw != "" {
		c, err := domain.DecodeCursor(raw)
		if err != nil {
			fields["cursor"] = "is malformed"
		}
		after = &c
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return nil, 0, false
	}
	return after, limit, true
}

func writePage(w http.ResponseWriter, p application.Page, view func(domain.Follow) edgeView) {
	res := listResponse{Data: make([]edgeView, len(p.Follows))}
	for i, f := range p.Follows {
		res.Data[i] = view(f)
	}
	if p.Next != nil {
		c := p.Next.Encode()
		res.Pagination = pagination{NextCursor: &c, HasMore: true}
	}
	httpserver.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) followers(t domain.TargetType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		after, limit, ok := pageParams(w, r, map[string]any{})
		if !ok {
			return
		}
		p, err := h.app.Followers(r.Context(), domain.Target{Type: t, ID: id}, after, limit)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writePage(w, p, func(f domain.Follow) edgeView {
			return edgeView{UserID: f.FollowerID.String(), FollowedAt: f.FollowedAt.UTC()}
		})
	}
}

// targetType reads ?type=user|artist (default user) into fields on error.
func targetType(r *http.Request, fields map[string]any) domain.TargetType {
	raw := r.URL.Query().Get("type")
	if raw == "" {
		return domain.TargetUser
	}
	t, ok := domain.ParseTargetType(raw)
	if !ok {
		fields["type"] = "must be user or artist"
	}
	return t
}

func (h *Handler) following(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	fields := map[string]any{}
	t := targetType(r, fields)
	after, limit, ok := pageParams(w, r, fields)
	if !ok {
		return
	}
	p, err := h.app.Following(r.Context(), id, t, after, limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writePage(w, p, func(f domain.Follow) edgeView {
		v := edgeView{FollowedAt: f.FollowedAt.UTC()}
		if t == domain.TargetArtist {
			v.ArtistID = f.Target.ID.String()
		} else {
			v.UserID = f.Target.ID.String()
		}
		return v
	})
}

type countsView struct {
	Followers        int64  `json:"followers"`
	FollowingUsers   *int64 `json:"followingUsers,omitempty"`
	FollowingArtists *int64 `json:"followingArtists,omitempty"`
}

// counts serves the profile/artist header counters; artists only have followers.
func (h *Handler) counts(t domain.TargetType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		c, err := h.app.Counts(r.Context(), domain.Target{Type: t, ID: id})
		if err != nil {
			h.fail(w, r, err)
			return
		}
		v := countsView{Followers: c.Followers}
		if t == domain.TargetUser {
			v.FollowingUsers, v.FollowingArtists = &c.FollowingUsers, &c.FollowingArtists
		}
		httpserver.WriteJSON(w, http.StatusOK, v)
	}
}

// contains answers "which of these do I follow?" for up to 100 IDs.
func (h *Handler) contains(w http.ResponseWriter, r *http.Request, user uuid.UUID) {
	fields := map[string]any{}
	t := targetType(r, fields)
	raw := strings.Split(r.URL.Query().Get("ids"), ",")
	ids := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		id, err := uuid.Parse(s)
		if err != nil {
			fields["ids"] = "must be comma-separated UUIDs"
			break
		}
		ids = append(ids, id)
	}
	if _, bad := fields["ids"]; !bad && (len(ids) == 0 || len(ids) > domain.MaxContains) {
		fields["ids"] = "must list 1 to 100 IDs"
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	followed, err := h.app.Contains(r.Context(), user, t, ids)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]string, len(followed))
	for i, id := range followed {
		out[i] = id.String()
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "social request failed", "error", err)
	httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Social is temporarily unavailable", nil)
}
