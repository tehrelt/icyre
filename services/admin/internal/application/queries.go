package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// ListTracks returns tracks from Catalog for moderators and admins.
func (s *Service) ListTracks(ctx context.Context, actor domain.Actor, q domain.TrackListQuery) (domain.Page[domain.TrackSummary], error) {
	if !actor.CanModerateTracks() {
		return domain.Page[domain.TrackSummary]{}, domain.ErrForbidden
	}
	q.Limit = clampLimit(q.Limit)
	return s.catalog.ListTracks(ctx, q)
}

// ListUsers returns users from Auth for admins only.
func (s *Service) ListUsers(ctx context.Context, actor domain.Actor, q domain.UserListQuery) (domain.Page[domain.UserSummary], error) {
	if !actor.CanModerateUsers() {
		return domain.Page[domain.UserSummary]{}, domain.ErrForbidden
	}
	q.Limit = clampLimit(q.Limit)
	return s.auth.ListUsers(ctx, q)
}

// ListIngestJobs returns upload sessions from Media Ingest for moderators and admins.
func (s *Service) ListIngestJobs(ctx context.Context, actor domain.Actor, q domain.IngestListQuery) (domain.Page[domain.IngestUpload], error) {
	if !actor.CanModerateTracks() {
		return domain.Page[domain.IngestUpload]{}, domain.ErrForbidden
	}
	q.Limit = clampLimit(q.Limit)
	return s.mediaIngest.ListUploads(ctx, q)
}

// ListTranscodeJobs returns projected transcode jobs for moderators and admins.
func (s *Service) ListTranscodeJobs(ctx context.Context, actor domain.Actor, filter domain.TranscodeJobFilter) (domain.Page[domain.TranscodeJob], error) {
	if !actor.CanModerateTracks() {
		return domain.Page[domain.TranscodeJob]{}, domain.ErrForbidden
	}
	limit := clampLimit(filter.Limit)
	filter.Limit = limit + 1
	rows, err := s.jobs.ListTranscodeJobs(ctx, filter)
	if err != nil {
		return domain.Page[domain.TranscodeJob]{}, err
	}
	hasMore := len(rows) > limit
	var nextCursor *string
	if hasMore {
		rows = rows[:limit]
		last := rows[limit-1]
		c := EncodeTimeIDCursor(last.EventAt, last.UploadID)
		nextCursor = &c
	}
	return domain.Page[domain.TranscodeJob]{Items: rows, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// ListAuditLogs returns immutable audit log entries for admins only.
func (s *Service) ListAuditLogs(ctx context.Context, actor domain.Actor, filter domain.AuditFilter) (domain.Page[domain.AuditEntry], error) {
	if !actor.CanViewAuditLogs() {
		return domain.Page[domain.AuditEntry]{}, domain.ErrForbidden
	}
	limit := clampLimit(filter.Limit)
	filter.Limit = limit + 1
	rows, err := s.audit.ListAuditLogs(ctx, filter)
	if err != nil {
		return domain.Page[domain.AuditEntry]{}, err
	}
	hasMore := len(rows) > limit
	var nextCursor *string
	if hasMore {
		rows = rows[:limit]
		last := rows[limit-1]
		c := EncodeTimeIDCursor(last.CreatedAt, last.ID)
		nextCursor = &c
	}
	return domain.Page[domain.AuditEntry]{Items: rows, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageSize
	}
	if limit > MaxPageSize {
		return MaxPageSize
	}
	return limit
}

type timeIDCursor struct {
	T string `json:"t"`
	I string `json:"i"`
}

// EncodeTimeIDCursor encodes a (timestamp, uuid) keyset cursor.
func EncodeTimeIDCursor(t time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(timeIDCursor{T: t.UTC().Format(time.RFC3339Nano), I: id.String()})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeTimeIDCursor decodes a (timestamp, uuid) keyset cursor.
func DecodeTimeIDCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	var c timeIDCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	t, err := time.Parse(time.RFC3339Nano, c.T)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(c.I)
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, errors.New("invalid cursor id")
	}
	return t, id, nil
}
