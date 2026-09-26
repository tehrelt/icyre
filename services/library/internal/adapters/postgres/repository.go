// Package postgres stores the library with pgx.
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

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/library/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by Library.
const Schema = "library"

// OutboxTable holds library.events messages until the relay sends them.
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

// Save inserts the item or returns the existing one untouched.
func (r *Repository) Save(ctx context.Context, it domain.Item) (domain.Change, error) {
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO library.items (user_id, kind, entity_id, saved_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, kind, entity_id) DO NOTHING
		RETURNING saved_at`, it.UserID, it.Kind, it.EntityID, it.SavedAt).Scan(&it.SavedAt)
	if err == nil {
		return domain.Change{Item: it, Changed: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Change{}, fmt.Errorf("save: %w", err)
	}
	// Already saved: report the original time.
	err = platformpg.Conn(ctx, r.pool).QueryRow(ctx, `SELECT saved_at FROM library.items WHERE user_id = $1 AND kind = $2 AND entity_id = $3`,
		it.UserID, it.Kind, it.EntityID).Scan(&it.SavedAt)
	if err != nil {
		return domain.Change{}, fmt.Errorf("save: read existing: %w", err)
	}
	return domain.Change{Item: it}, nil
}

// Remove deletes the item if present.
func (r *Repository) Remove(ctx context.Context, userID uuid.UUID, kind domain.Kind, id uuid.UUID, at time.Time) (domain.Change, error) {
	it := domain.Item{UserID: userID, Kind: kind, EntityID: id, SavedAt: at}
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `DELETE FROM library.items WHERE user_id = $1 AND kind = $2 AND entity_id = $3`, userID, kind, id)
	if err != nil {
		return domain.Change{}, fmt.Errorf("remove: %w", err)
	}
	return domain.Change{Item: it, Changed: tag.RowsAffected() == 1}, nil
}

// List returns items newest first after the cursor.
func (r *Repository) List(ctx context.Context, userID uuid.UUID, kind domain.Kind, after *domain.Cursor, limit int) ([]domain.Item, error) {
	var (
		afterAt *time.Time
		afterID *uuid.UUID
	)
	if after != nil {
		afterAt, afterID = &after.SavedAt, &after.EntityID
	}
	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT entity_id, saved_at FROM library.items
		WHERE user_id = $1 AND kind = $2
		  AND ($3::timestamptz IS NULL OR (saved_at, entity_id) < ($3, $4::uuid))
		ORDER BY saved_at DESC, entity_id DESC
		LIMIT $5`, userID, kind, afterAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Item, error) {
		it := domain.Item{UserID: userID, Kind: kind}
		err := row.Scan(&it.EntityID, &it.SavedAt)
		return it, err
	})
}

// Contains returns which ids are saved.
func (r *Repository) Contains(ctx context.Context, userID uuid.UUID, kind domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `SELECT entity_id FROM library.items WHERE user_id = $1 AND kind = $2 AND entity_id = ANY($3)`, userID, kind, ids)
	if err != nil {
		return nil, fmt.Errorf("contains: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// Counts counts saved items per kind.
func (r *Repository) Counts(ctx context.Context, userID uuid.UUID) (domain.Counts, error) {
	var c domain.Counts
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE kind = 'track'), count(*) FILTER (WHERE kind = 'album')
		FROM library.items WHERE user_id = $1`, userID).Scan(&c.Tracks, &c.Albums)
	if err != nil {
		return c, fmt.Errorf("counts: %w", err)
	}
	return c, nil
}
