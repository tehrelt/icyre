// Package postgres stores playlists with pgx.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by Playlist.
const Schema = "playlist"

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

const selectPlaylist = `
	SELECT p.id, p.owner_id, p.title, p.created_at, p.updated_at,
	       (SELECT count(*) FROM playlist.playlist_tracks t WHERE t.playlist_id = p.id)
	FROM playlist.playlists p`

func scan(row pgx.Row) (domain.Playlist, error) {
	var p domain.Playlist
	err := row.Scan(&p.ID, &p.OwnerID, &p.Title, &p.CreatedAt, &p.UpdatedAt, &p.TrackCount)
	return p, err
}

// Create inserts a playlist.
func (r *Repository) Create(ctx context.Context, p domain.Playlist) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO playlist.playlists (id, owner_id, title, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		p.ID, p.OwnerID, p.Title, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create playlist: %w", err)
	}
	return nil
}

// Get loads one playlist.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Playlist, error) {
	p, err := scan(r.pool.QueryRow(ctx, selectPlaylist+` WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	if err != nil {
		return p, fmt.Errorf("get playlist: %w", err)
	}
	return p, nil
}

// ByOwner lists an owner's playlists, recently changed first.
func (r *Repository) ByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Playlist, error) {
	rows, err := r.pool.Query(ctx, selectPlaylist+` WHERE p.owner_id = $1 ORDER BY p.updated_at DESC, p.id DESC LIMIT 200`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list playlists: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Playlist, error) { return scan(row) })
}

// Tracks returns a playlist's tracks by position.
func (r *Repository) Tracks(ctx context.Context, id uuid.UUID) ([]domain.Track, error) {
	rows, err := r.pool.Query(ctx, `SELECT track_id, position, added_by, added_at FROM playlist.playlist_tracks WHERE playlist_id = $1 ORDER BY position`, id)
	if err != nil {
		return nil, fmt.Errorf("playlist tracks: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Track, error) {
		var t domain.Track
		err := row.Scan(&t.TrackID, &t.Position, &t.AddedBy, &t.AddedAt)
		return t, err
	})
}

// AppendTrack adds a track after the last position in one transaction; the
// playlist row lock serializes concurrent appends.
func (r *Repository) AppendTrack(ctx context.Context, id uuid.UUID, t domain.Track) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM playlist.playlists WHERE id = $1 FOR UPDATE`, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by, added_at)
			SELECT $1, $2, coalesce(max(position), 0) + 1, $3, $4 FROM playlist.playlist_tracks WHERE playlist_id = $1
			ON CONFLICT (playlist_id, track_id) DO NOTHING`, id, t.TrackID, t.AddedBy, t.AddedAt)
		if err != nil {
			return fmt.Errorf("append track: %w", err)
		}
		if tag.RowsAffected() == 1 {
			_, err = tx.Exec(ctx, `UPDATE playlist.playlists SET updated_at = $2 WHERE id = $1`, id, t.AddedAt)
		}
		return err
	})
}

// RemoveTrack deletes a track; positions keep their order (gaps are fine).
func (r *Repository) RemoveTrack(ctx context.Context, id, trackID uuid.UUID, at time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1 AND track_id = $2`, id, trackID)
		if err != nil {
			return fmt.Errorf("remove track: %w", err)
		}
		if tag.RowsAffected() == 1 {
			_, err = tx.Exec(ctx, `UPDATE playlist.playlists SET updated_at = $2 WHERE id = $1`, id, at)
		}
		return err
	})
}
