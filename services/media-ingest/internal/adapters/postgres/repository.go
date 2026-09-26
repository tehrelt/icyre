// Package postgres stores upload sessions with pgx.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by Media Ingest.
const Schema = "media"

// OutboxTable holds media.events messages until the relay sends them.
const OutboxTable = Schema + ".outbox"

// Migrations returns the embedded migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// Repository implements application.Repository.
type Repository struct{ pool *pgxpool.Pool }

// New returns a Repository.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Create inserts a pending session.
func (r *Repository) Create(ctx context.Context, u domain.Upload) error {
	_, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO media.uploads (id, track_id, uploader_id, content_type, size_bytes, sha256, object_key, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		u.ID, u.TrackID, u.UploaderID, u.ContentType, u.SizeBytes, u.SHA256, u.Key, u.Status, u.CreatedAt, u.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create upload: %w", err)
	}
	return nil
}

// Get returns a session or domain.ErrNotFound.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Upload, error) {
	var (
		u      domain.Upload
		reason *string
	)
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, track_id, uploader_id, content_type, size_bytes, sha256, object_key, status, failure_reason, created_at, expires_at, completed_at
		FROM media.uploads WHERE id = $1`, id).
		Scan(&u.ID, &u.TrackID, &u.UploaderID, &u.ContentType, &u.SizeBytes, &u.SHA256, &u.Key, &u.Status, &reason, &u.CreatedAt, &u.ExpiresAt, &u.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Upload{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Upload{}, fmt.Errorf("get upload: %w", err)
	}
	if reason != nil {
		u.FailureReason = *reason
	}
	return u, nil
}

// Settle stores the final status only while the session is pending, so
// concurrent completes settle (and publish) once.
func (r *Repository) Settle(ctx context.Context, u domain.Upload) (bool, error) {
	var reason *string
	if u.FailureReason != "" {
		reason = &u.FailureReason
	}
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		UPDATE media.uploads SET status = $2, failure_reason = $3, completed_at = $4
		WHERE id = $1 AND status = 'PENDING'`, u.ID, u.Status, reason, u.CompletedAt)
	if err != nil {
		return false, fmt.Errorf("settle upload: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
