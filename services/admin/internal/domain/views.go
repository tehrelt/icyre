package domain

import (
	"time"

	"github.com/google/uuid"
)

// TranscodeStatus represents the projected state of a media transcode job.
type TranscodeStatus string

const (
	TranscodeQueued    TranscodeStatus = "QUEUED"
	TranscodeSucceeded TranscodeStatus = "SUCCEEDED"
	TranscodeFailed    TranscodeStatus = "FAILED"
)

// Valid reports whether s is a supported transcode job status.
func (s TranscodeStatus) Valid() bool {
	return s == TranscodeQueued || s == TranscodeSucceeded || s == TranscodeFailed
}

// TranscodeJob is the admin projection of media pipeline transcode events.
type TranscodeJob struct {
	UploadID      uuid.UUID
	TrackID       uuid.UUID
	Status        TranscodeStatus
	FailureReason string
	EventAt       time.Time
	ProjectedAt   time.Time
}

// IngestUpload is a read-only view of an upload session from Media Ingest.
type IngestUpload struct {
	ID            uuid.UUID
	TrackID       uuid.UUID
	Status        string
	FailureReason *string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	CompletedAt   *time.Time
}

// TrackSummary is a read-only view of a track from Catalog.
type TrackSummary struct {
	ID          uuid.UUID
	AlbumID     uuid.UUID
	ArtistIDs   []uuid.UUID
	Title       string
	DurationMs  int64
	TrackNumber int
	DiscNumber  int
	Explicit    bool
	ISRC        *string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserSummary is a read-only view of an account from Auth.
type UserSummary struct {
	ID        uuid.UUID
	Roles     []string
	Blocked   bool
	BlockedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Page is a cursor-paginated slice of items.
type Page[T any] struct {
	Items      []T
	NextCursor *string
	HasMore    bool
}

// TrackListQuery filters tracks fetched from Catalog.
type TrackListQuery struct {
	Status string
	Limit  int
	Cursor string
}

// UserListQuery filters users fetched from Auth.
type UserListQuery struct {
	Blocked *bool
	Limit   int
	Cursor  string
}

// IngestListQuery filters upload sessions fetched from Media Ingest.
type IngestListQuery struct {
	TrackID *uuid.UUID
	Status  string
	Limit   int
	Cursor  string
}

// TranscodeJobFilter filters projected transcode jobs in PostgreSQL.
type TranscodeJobFilter struct {
	TrackID       *uuid.UUID
	Status        *TranscodeStatus
	AfterEventAt  *time.Time
	AfterUploadID *uuid.UUID
	Limit         int
}

// AuditFilter filters immutable audit log entries in PostgreSQL.
type AuditFilter struct {
	ActorID        *uuid.UUID
	TargetType     *TargetType
	TargetID       *uuid.UUID
	Action         *Action
	Phase          *AuditPhase
	AfterCreatedAt *time.Time
	AfterID        *uuid.UUID
	Limit          int
}
