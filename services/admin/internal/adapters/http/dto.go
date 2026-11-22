package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

type moderateRequest struct {
	Blocked *bool  `json:"blocked"`
	Reason  string `json:"reason"`
}

type operationResponse struct {
	ID                 string         `json:"id"`
	ActorID            string         `json:"actorId"`
	Action             string         `json:"action"`
	TargetType         string         `json:"targetType"`
	TargetID           string         `json:"targetId"`
	DesiredBlocked     bool           `json:"desiredBlocked"`
	Reason             string         `json:"reason"`
	Status             string         `json:"status"`
	ResultCode         string         `json:"resultCode,omitempty"`
	BeforeState        map[string]any `json:"beforeState"`
	AfterState         map[string]any `json:"afterState"`
	RevocationsPending bool           `json:"revocationsPending"`
	RequestID          string         `json:"requestId,omitempty"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	CompletedAt        *time.Time     `json:"completedAt,omitempty"`
}

type trackResponse struct {
	ID          string    `json:"id"`
	AlbumID     string    `json:"albumId"`
	ArtistIDs   []string  `json:"artistIds"`
	Title       string    `json:"title"`
	DurationMs  int64     `json:"durationMs"`
	TrackNumber int       `json:"trackNumber"`
	DiscNumber  int       `json:"discNumber"`
	Explicit    bool      `json:"explicit"`
	ISRC        *string   `json:"isrc"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type userResponse struct {
	ID        string     `json:"id"`
	Roles     []string   `json:"roles"`
	Blocked   bool       `json:"blocked"`
	BlockedAt *time.Time `json:"blockedAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type ingestJobResponse struct {
	Kind          string     `json:"kind"`
	ID            string     `json:"id"`
	TrackID       string     `json:"trackId"`
	Status        string     `json:"status"`
	FailureReason *string    `json:"failureReason"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	CompletedAt   *time.Time `json:"completedAt"`
}

type transcodeJobResponse struct {
	Kind          string    `json:"kind"`
	UploadID      string    `json:"uploadId"`
	TrackID       string    `json:"trackId"`
	Status        string    `json:"status"`
	FailureReason *string   `json:"failureReason"`
	EventAt       time.Time `json:"eventAt"`
	ProjectedAt   time.Time `json:"projectedAt"`
}

type auditLogResponse struct {
	ID          string         `json:"id"`
	OperationID string         `json:"operationId"`
	ActorID     string         `json:"actorId"`
	Action      string         `json:"action"`
	TargetType  string         `json:"targetType"`
	TargetID    string         `json:"targetId"`
	Phase       string         `json:"phase"`
	Reason      string         `json:"reason"`
	Metadata    map[string]any `json:"metadata"`
	RequestID   string         `json:"requestId,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type pagination struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type listResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination pagination `json:"pagination"`
}

func toOperationResponse(op domain.Operation) operationResponse {
	var completedAt *time.Time
	if op.CompletedAt != nil {
		t := op.CompletedAt.UTC()
		completedAt = &t
	}
	return operationResponse{
		ID:                 op.ID.String(),
		ActorID:            op.ActorID.String(),
		Action:             string(op.Action),
		TargetType:         string(op.TargetType),
		TargetID:           op.TargetID.String(),
		DesiredBlocked:     op.DesiredBlocked,
		Reason:             op.Reason,
		Status:             string(op.Status),
		ResultCode:         op.ResultCode,
		BeforeState:        domain.CloneMap(op.BeforeState),
		AfterState:         domain.CloneMap(op.AfterState),
		RevocationsPending: op.RevocationsPending,
		RequestID:          op.RequestID,
		CreatedAt:          op.CreatedAt.UTC(),
		UpdatedAt:          op.UpdatedAt.UTC(),
		CompletedAt:        completedAt,
	}
}

func toTrackResponse(t domain.TrackSummary) trackResponse {
	return trackResponse{
		ID:          t.ID.String(),
		AlbumID:     t.AlbumID.String(),
		ArtistIDs:   idStrings(t.ArtistIDs),
		Title:       t.Title,
		DurationMs:  t.DurationMs,
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
		Explicit:    t.Explicit,
		ISRC:        t.ISRC,
		Status:      t.Status,
		CreatedAt:   t.CreatedAt.UTC(),
		UpdatedAt:   t.UpdatedAt.UTC(),
	}
}

func toUserResponse(u domain.UserSummary) userResponse {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	var blockedAt *time.Time
	if u.BlockedAt != nil {
		t := u.BlockedAt.UTC()
		blockedAt = &t
	}
	return userResponse{
		ID:        u.ID.String(),
		Roles:     roles,
		Blocked:   u.Blocked,
		BlockedAt: blockedAt,
		CreatedAt: u.CreatedAt.UTC(),
		UpdatedAt: u.UpdatedAt.UTC(),
	}
}

func toIngestJobResponse(u domain.IngestUpload) ingestJobResponse {
	var completedAt *time.Time
	if u.CompletedAt != nil {
		t := u.CompletedAt.UTC()
		completedAt = &t
	}
	return ingestJobResponse{
		Kind:          "ingest",
		ID:            u.ID.String(),
		TrackID:       u.TrackID.String(),
		Status:        u.Status,
		FailureReason: u.FailureReason,
		CreatedAt:     u.CreatedAt.UTC(),
		ExpiresAt:     u.ExpiresAt.UTC(),
		CompletedAt:   completedAt,
	}
}

func toTranscodeJobResponse(j domain.TranscodeJob) transcodeJobResponse {
	var reason *string
	if j.FailureReason != "" {
		r := j.FailureReason
		reason = &r
	}
	return transcodeJobResponse{
		Kind:          "transcode",
		UploadID:      j.UploadID.String(),
		TrackID:       j.TrackID.String(),
		Status:        string(j.Status),
		FailureReason: reason,
		EventAt:       j.EventAt.UTC(),
		ProjectedAt:   j.ProjectedAt.UTC(),
	}
}

func toAuditLogResponse(e domain.AuditEntry) auditLogResponse {
	return auditLogResponse{
		ID:          e.ID.String(),
		OperationID: e.OperationID.String(),
		ActorID:     e.ActorID.String(),
		Action:      string(e.Action),
		TargetType:  string(e.TargetType),
		TargetID:    e.TargetID.String(),
		Phase:       string(e.Phase),
		Reason:      e.Reason,
		Metadata:    domain.CloneMap(e.Metadata),
		RequestID:   e.RequestID,
		CreatedAt:   e.CreatedAt.UTC(),
	}
}

func toListResponse[S, D any](page domain.Page[S], mapFn func(S) D) listResponse[D] {
	data := make([]D, len(page.Items))
	for i, item := range page.Items {
		data[i] = mapFn(item)
	}
	return listResponse[D]{
		Data:       data,
		Pagination: pagination{NextCursor: page.NextCursor, HasMore: page.HasMore},
	}
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
