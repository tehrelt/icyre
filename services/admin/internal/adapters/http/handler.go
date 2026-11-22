// Package http exposes the Admin Service REST API under /api/v1/admin/*.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// AdminApp is the application layer subset used by HTTP handlers.
type AdminApp interface {
	ListTracks(ctx context.Context, actor domain.Actor, q domain.TrackListQuery) (domain.Page[domain.TrackSummary], error)
	ModerateTrack(ctx context.Context, actor domain.Actor, idempotencyKey string, trackID uuid.UUID, blocked bool, reason string) (domain.Operation, error)
	ListUsers(ctx context.Context, actor domain.Actor, q domain.UserListQuery) (domain.Page[domain.UserSummary], error)
	ModerateUser(ctx context.Context, actor domain.Actor, idempotencyKey string, userID uuid.UUID, blocked bool, reason string) (domain.Operation, error)
	ListIngestJobs(ctx context.Context, actor domain.Actor, q domain.IngestListQuery) (domain.Page[domain.IngestUpload], error)
	ListTranscodeJobs(ctx context.Context, actor domain.Actor, filter domain.TranscodeJobFilter) (domain.Page[domain.TranscodeJob], error)
	ListAuditLogs(ctx context.Context, actor domain.Actor, filter domain.AuditFilter) (domain.Page[domain.AuditEntry], error)
	GetOperation(ctx context.Context, actor domain.Actor, id uuid.UUID) (domain.Operation, error)
}

// Handler serves /api/v1/admin/* routes.
type Handler struct {
	app      AdminApp
	verifier *authn.Verifier
	log      *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app AdminApp, verifier *authn.Verifier, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{app: app, verifier: verifier, log: log}
}

// Register mounts all admin routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	modOrAdmin := []string{authn.RoleModerator, authn.RoleAdmin}
	adminOnly := []string{authn.RoleAdmin}

	mux.Handle("GET /api/v1/admin/tracks", h.protect(h.listTracks, modOrAdmin...))
	mux.Handle("PATCH /api/v1/admin/tracks/{id}", h.protect(h.moderateTrack, modOrAdmin...))
	mux.Handle("GET /api/v1/admin/users", h.protect(h.listUsers, adminOnly...))
	mux.Handle("PATCH /api/v1/admin/users/{id}", h.protect(h.moderateUser, adminOnly...))
	mux.Handle("GET /api/v1/admin/jobs", h.protect(h.listJobs, modOrAdmin...))
	mux.Handle("GET /api/v1/admin/audit-logs", h.protect(h.listAuditLogs, adminOnly...))
	mux.Handle("GET /api/v1/admin/operations/{id}", h.protect(h.getOperation, modOrAdmin...))
}

type actorHandler func(w http.ResponseWriter, r *http.Request, actor domain.Actor)

func (h *Handler) protect(next actorHandler, roles ...string) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := authn.FromContext(r.Context())
		id, err := uuid.Parse(p.UserID)
		if !ok || err != nil || id == uuid.Nil {
			httpserver.WriteError(w, r, http.StatusUnauthorized, authn.CodeUnauthenticated, "Authentication required", nil)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		actor := domain.Actor{
			ID:        id,
			Moderator: p.HasRole(authn.RoleModerator),
			Admin:     p.HasRole(authn.RoleAdmin),
		}
		next(w, r, actor)
	})
	return h.verifier.RequiredStrict(authn.RequireAnyRole(roles...)(inner))
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, h.log, err)
}

func (h *Handler) listTracks(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	q, err := parseTrackQuery(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.app.ListTracks(r.Context(), actor, q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toListResponse(page, toTrackResponse))
}

func (h *Handler) moderateTrack(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	id, key, req, err := parseModerateRequest(w, r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	op, err := h.app.ModerateTrack(r.Context(), actor, key, id, *req.Blocked, req.Reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeOperationResponse(w, op)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	q, err := parseUserQuery(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.app.ListUsers(r.Context(), actor, q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toListResponse(page, toUserResponse))
}

func (h *Handler) moderateUser(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	id, key, req, err := parseModerateRequest(w, r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	op, err := h.app.ModerateUser(r.Context(), actor, key, id, *req.Blocked, req.Reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeOperationResponse(w, op)
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	switch kind {
	case "ingest":
		q, err := parseIngestQuery(r)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		page, err := h.app.ListIngestJobs(r.Context(), actor, q)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, toListResponse(page, toIngestJobResponse))
	case "transcode":
		filter, err := parseTranscodeQuery(r)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		page, err := h.app.ListTranscodeJobs(r.Context(), actor, filter)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, toListResponse(page, toTranscodeJobResponse))
	default:
		h.fail(w, r, invalidFields(map[string]string{"kind": "must be one of ingest, transcode"}))
	}
}

func (h *Handler) listAuditLogs(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	filter, err := parseAuditQuery(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.app.ListAuditLogs(r.Context(), actor, filter)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toListResponse(page, toAuditLogResponse))
}

func (h *Handler) getOperation(w http.ResponseWriter, r *http.Request, actor domain.Actor) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	op, err := h.app.GetOperation(r.Context(), actor, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toOperationResponse(op))
}

func parseModerateRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, moderateRequest, error) {
	id, err := pathID(r)
	if err != nil {
		return uuid.Nil, "", moderateRequest{}, err
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return uuid.Nil, "", moderateRequest{}, &requestError{
			status:  http.StatusUnsupportedMediaType,
			code:    httpserver.CodeUnsupportedType,
			message: "Content-Type must be application/json",
		}
	}
	var req moderateRequest
	if err := httpserver.DecodeJSON(w, r, &req); err != nil {
		return uuid.Nil, "", moderateRequest{}, badRequest(err.Error())
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	fields := map[string]string{}
	if key == "" {
		fields["Idempotency-Key"] = "header is required"
	}
	if req.Blocked == nil {
		fields["blocked"] = "is required"
	}
	if len(fields) > 0 {
		return uuid.Nil, "", moderateRequest{}, invalidFields(fields)
	}
	return id, key, req, nil
}

func writeOperationResponse(w http.ResponseWriter, op domain.Operation) {
	if op.Status == domain.StatusPending {
		w.Header().Set("Location", "/api/v1/admin/operations/"+op.ID.String())
		httpserver.WriteJSON(w, http.StatusAccepted, toOperationResponse(op))
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toOperationResponse(op))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, badRequest("path parameter id must be a UUID")
	}
	return id, nil
}
