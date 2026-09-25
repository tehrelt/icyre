// Package postgres stores the listening history with pgx.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tehrelt/icyre/services/history/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by Listening History.
const Schema = "history"

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

// Record inserts a listen once per playback.
func (r *Repository) Record(ctx context.Context, l domain.Listen) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO history.listens (playback_id, user_id, track_id, source, duration_ms, listened_ms, played_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (playback_id) DO NOTHING`,
		l.PlaybackID, l.UserID, l.TrackID, l.Source, l.DurationMs, l.ListenedMs, l.PlayedAt)
	if err != nil {
		return false, fmt.Errorf("record: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Tracks returns listens newest first after the cursor.
func (r *Repository) Tracks(ctx context.Context, userID uuid.UUID, after *domain.Cursor, limit int) ([]domain.Listen, error) {
	var (
		afterAt *time.Time
		afterID *uuid.UUID
	)
	if after != nil {
		afterAt, afterID = &after.PlayedAt, &after.PlaybackID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT playback_id, track_id, source, duration_ms, listened_ms, played_at FROM history.listens
		WHERE user_id = $1 AND ($2::timestamptz IS NULL OR (played_at, playback_id) < ($2, $3::uuid))
		ORDER BY played_at DESC, playback_id DESC
		LIMIT $4`, userID, afterAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("tracks: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Listen, error) {
		l := domain.Listen{UserID: userID}
		err := row.Scan(&l.PlaybackID, &l.TrackID, &l.Source, &l.DurationMs, &l.ListenedMs, &l.PlayedAt)
		return l, err
	})
}

// RecentSources returns distinct sources by their latest listen.
func (r *Repository) RecentSources(ctx context.Context, userID uuid.UUID, limit int) ([]domain.RecentSource, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source, max(played_at) AS last FROM history.listens
		WHERE user_id = $1 AND source <> ''
		GROUP BY source ORDER BY last DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent sources: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RecentSource, error) {
		var s domain.RecentSource
		err := row.Scan(&s.Source, &s.PlayedAt)
		return s, err
	})
}
