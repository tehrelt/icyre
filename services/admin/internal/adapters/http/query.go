package http

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/admin/internal/application"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

func parseLimit(q url.Values, fields map[string]string) int {
	raw := q.Get("limit")
	if raw == "" {
		return application.DefaultPageSize
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > application.MaxPageSize {
		fields["limit"] = "must be between 1 and " + strconv.Itoa(application.MaxPageSize)
		return 0
	}
	return n
}

func parseOptionalUUID(q url.Values, key string, fields map[string]string) *uuid.UUID {
	raw := q.Get(key)
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		fields[key] = "must be a valid UUID"
		return nil
	}
	return &id
}

func parseTrackQuery(r *http.Request) (domain.TrackListQuery, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	limit := parseLimit(q, fields)
	status := strings.ToUpper(strings.TrimSpace(q.Get("status")))
	if status != "" {
		switch status {
		case "DRAFT", "PROCESSING", "READY", "BLOCKED", "DELETED":
		default:
			fields["status"] = "must be one of DRAFT, PROCESSING, READY, BLOCKED, DELETED"
		}
	}
	if len(fields) > 0 {
		return domain.TrackListQuery{}, invalidFields(fields)
	}
	return domain.TrackListQuery{Status: status, Limit: limit, Cursor: q.Get("cursor")}, nil
}

func parseUserQuery(r *http.Request) (domain.UserListQuery, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	limit := parseLimit(q, fields)
	var blocked *bool
	if raw := q.Get("blocked"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			fields["blocked"] = "must be a boolean"
		} else {
			blocked = &b
		}
	}
	if len(fields) > 0 {
		return domain.UserListQuery{}, invalidFields(fields)
	}
	return domain.UserListQuery{Blocked: blocked, Limit: limit, Cursor: q.Get("cursor")}, nil
}

func parseIngestQuery(r *http.Request) (domain.IngestListQuery, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	limit := parseLimit(q, fields)
	trackID := parseOptionalUUID(q, "trackId", fields)
	status := strings.ToUpper(strings.TrimSpace(q.Get("status")))
	if status != "" {
		switch status {
		case "PENDING", "COMPLETED", "FAILED":
		default:
			fields["status"] = "must be one of PENDING, COMPLETED, FAILED"
		}
	}
	if len(fields) > 0 {
		return domain.IngestListQuery{}, invalidFields(fields)
	}
	return domain.IngestListQuery{TrackID: trackID, Status: status, Limit: limit, Cursor: q.Get("cursor")}, nil
}

func parseTranscodeQuery(r *http.Request) (domain.TranscodeJobFilter, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	limit := parseLimit(q, fields)
	trackID := parseOptionalUUID(q, "trackId", fields)
	var status *domain.TranscodeStatus
	if raw := strings.ToUpper(strings.TrimSpace(q.Get("status"))); raw != "" {
		s := domain.TranscodeStatus(raw)
		if !s.Valid() {
			fields["status"] = "must be one of QUEUED, SUCCEEDED, FAILED"
		} else {
			status = &s
		}
	}
	if len(fields) > 0 {
		return domain.TranscodeJobFilter{}, invalidFields(fields)
	}
	filter := domain.TranscodeJobFilter{TrackID: trackID, Status: status, Limit: limit}
	if raw := q.Get("cursor"); raw != "" {
		t, id, err := application.DecodeTimeIDCursor(raw)
		if err != nil {
			return domain.TranscodeJobFilter{}, invalidCursor()
		}
		filter.AfterEventAt, filter.AfterUploadID = &t, &id
	}
	return filter, nil
}

func parseAuditQuery(r *http.Request) (domain.AuditFilter, error) {
	q := r.URL.Query()
	fields := map[string]string{}
	limit := parseLimit(q, fields)
	actorID := parseOptionalUUID(q, "actorId", fields)
	targetID := parseOptionalUUID(q, "targetId", fields)
	targetType := parseAuditTargetType(q, fields)
	action := parseAuditAction(q, fields)
	phase := parseAuditPhase(q, fields)
	if len(fields) > 0 {
		return domain.AuditFilter{}, invalidFields(fields)
	}
	filter := domain.AuditFilter{
		ActorID: actorID, TargetType: targetType, TargetID: targetID,
		Action: action, Phase: phase, Limit: limit,
	}
	if raw := q.Get("cursor"); raw != "" {
		t, id, err := application.DecodeTimeIDCursor(raw)
		if err != nil {
			return domain.AuditFilter{}, invalidCursor()
		}
		filter.AfterCreatedAt, filter.AfterID = &t, &id
	}
	return filter, nil
}

func parseAuditTargetType(q url.Values, fields map[string]string) *domain.TargetType {
	raw := strings.ToLower(strings.TrimSpace(q.Get("targetType")))
	if raw == "" {
		return nil
	}
	t := domain.TargetType(raw)
	if !t.Valid() {
		fields["targetType"] = "must be one of track, user"
		return nil
	}
	return &t
}

func parseAuditAction(q url.Values, fields map[string]string) *domain.Action {
	raw := strings.ToLower(strings.TrimSpace(q.Get("action")))
	if raw == "" {
		return nil
	}
	a := domain.Action(raw)
	if !a.Valid() {
		fields["action"] = "must be one of track.block, track.unblock, user.block, user.unblock"
		return nil
	}
	return &a
}

func parseAuditPhase(q url.Values, fields map[string]string) *domain.AuditPhase {
	raw := strings.ToLower(strings.TrimSpace(q.Get("phase")))
	if raw == "" {
		return nil
	}
	p := domain.AuditPhase(raw)
	if !p.Valid() {
		fields["phase"] = "must be one of requested, applied, noop, rejected"
		return nil
	}
	return &p
}
