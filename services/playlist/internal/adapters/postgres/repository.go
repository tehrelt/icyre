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

// UpdateTitle renames a playlist.
func (r *Repository) UpdateTitle(ctx context.Context, id uuid.UUID, title string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE playlist.playlists SET title = $2, updated_at = $3 WHERE id = $1`, id, title, at)
	if err != nil {
		return fmt.Errorf("update playlist: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete removes a playlist; its tracks go with it (ON DELETE CASCADE).
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM playlist.playlists WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete playlist: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// lock takes the playlist row lock that serializes changes to its tracks.
func lock(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM playlist.playlists WHERE id = $1 FOR UPDATE`, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func touch(ctx context.Context, tx pgx.Tx, id uuid.UUID, at time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE playlist.playlists SET updated_at = $2 WHERE id = $1`, id, at)
	return err
}

// AppendTrack adds a track after the last position in one transaction; the
// playlist row lock serializes concurrent appends.
func (r *Repository) AppendTrack(ctx context.Context, id uuid.UUID, t domain.Track) (domain.Track, bool, error) {
	added := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, id); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by, added_at)
			SELECT $1, $2, coalesce(max(position), 0) + 1, $3, $4 FROM playlist.playlist_tracks WHERE playlist_id = $1
			ON CONFLICT (playlist_id, track_id) DO NOTHING
			RETURNING position`, id, t.TrackID, t.AddedBy, t.AddedAt).Scan(&t.Position)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("append track: %w", err)
		}
		added = true
		return touch(ctx, tx, id, t.AddedAt)
	})
	return t, added, err
}

// RemoveTrack deletes a track; positions keep their order (gaps are fine).
func (r *Repository) RemoveTrack(ctx context.Context, id, trackID uuid.UUID, at time.Time) (bool, error) {
	removed := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, id); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1 AND track_id = $2`, id, trackID)
		if err != nil {
			return fmt.Errorf("remove track: %w", err)
		}
		if removed = tag.RowsAffected() == 1; removed {
			return touch(ctx, tx, id, at)
		}
		return nil
	})
	return removed, err
}

// Reorder rewrites positions to 1..n in the given order under the playlist
// lock, after checking the order against the current tracks. The position
// uniqueness check is deferred to commit (migration 00002).
func (r *Repository) Reorder(ctx context.Context, id uuid.UUID, order []uuid.UUID, at time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lock(ctx, tx, id); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT track_id, position FROM playlist.playlist_tracks WHERE playlist_id = $1`, id)
		if err != nil {
			return fmt.Errorf("reorder: %w", err)
		}
		current, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Track, error) {
			var t domain.Track
			err := row.Scan(&t.TrackID, &t.Position)
			return t, err
		})
		if err != nil {
			return fmt.Errorf("reorder: %w", err)
		}
		if err := domain.CheckOrder(current, order); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE playlist.playlist_tracks t SET position = o.pos
			FROM unnest($2::uuid[]) WITH ORDINALITY AS o(track_id, pos)
			WHERE t.playlist_id = $1 AND t.track_id = o.track_id`, id, order); err != nil {
			return fmt.Errorf("reorder: %w", err)
		}
		return touch(ctx, tx, id, at)
	})
}
