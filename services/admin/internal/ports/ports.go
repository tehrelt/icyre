// Package ports defines the application boundaries for the Admin Service.
package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// Transactor runs a unit of work inside a database transaction.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// OperationRepository persists and queries moderation operations.
type OperationRepository interface {
	FindOrCreateOperation(ctx context.Context, op domain.Operation) (domain.Operation, bool, error)
	GetOperation(ctx context.Context, id uuid.UUID) (domain.Operation, error)
	FinalizeOperation(ctx context.Context, op domain.Operation) (domain.Operation, error)
	ListPendingOperations(ctx context.Context, limit int) ([]domain.Operation, error)
}

// AuditRepository appends and queries immutable audit log entries.
type AuditRepository interface {
	AppendAuditLog(ctx context.Context, entry domain.AuditEntry) error
	ListAuditLogs(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, error)
}

// TranscodeJobRepository stores and queries projected transcode jobs.
type TranscodeJobRepository interface {
	UpsertTranscodeJob(ctx context.Context, job domain.TranscodeJob) error
	ListTranscodeJobs(ctx context.Context, filter domain.TranscodeJobFilter) ([]domain.TranscodeJob, error)
}

// CatalogClient communicates with Catalog's internal moderation API.
type CatalogClient interface {
	ListTracks(ctx context.Context, q domain.TrackListQuery) (domain.Page[domain.TrackSummary], error)
	ModerateTrack(ctx context.Context, operationID, trackID uuid.UUID, blocked bool) (domain.TrackModerationReceipt, error)
}

// AuthClient communicates with Auth's internal moderation API.
type AuthClient interface {
	ListUsers(ctx context.Context, q domain.UserListQuery) (domain.Page[domain.UserSummary], error)
	ModerateUser(ctx context.Context, operationID, userID uuid.UUID, blocked bool) (domain.UserModerationReceipt, error)
}

// MediaIngestClient communicates with Media Ingest's internal moderation API.
type MediaIngestClient interface {
	ListUploads(ctx context.Context, q domain.IngestListQuery) (domain.Page[domain.IngestUpload], error)
}
