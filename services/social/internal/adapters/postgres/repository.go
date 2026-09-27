// Package postgres stores the social graph with pgx.
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
	"github.com/tehrelt/icyre/services/social/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by Social.
const Schema = "social"

// OutboxTable holds social.events messages until the relay sends them.
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

// Follow inserts the edge and bumps both counters in one statement, so an edge
// and its counters never diverge even outside a transaction. The follower's
// counter row is always a user row and never the target's (no self-follow).
func (r *Repository) Follow(ctx context.Context, f domain.Follow) (domain.Change, error) {
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO social.follows (follower_id, target_type, target_id, followed_at) VALUES ($1, $2, $3, $4)
			ON CONFLICT (follower_id, target_type, target_id) DO NOTHING
			RETURNING followed_at
		), target AS (
			INSERT INTO social.counters (subject_type, subject_id, followers)
			SELECT $2, $3, 1 FROM ins
			ON CONFLICT (subject_type, subject_id) DO UPDATE SET followers = social.counters.followers + 1
		), follower AS (
			INSERT INTO social.counters (subject_type, subject_id, following_users, following_artists)
			SELECT 'user', $1, ($2 = 'user')::int, ($2 = 'artist')::int FROM ins
			ON CONFLICT (subject_type, subject_id) DO UPDATE SET
				following_users   = social.counters.following_users + excluded.following_users,
				following_artists = social.counters.following_artists + excluded.following_artists
		)
		SELECT followed_at FROM ins`, f.FollowerID, f.Target.Type, f.Target.ID, f.FollowedAt).Scan(&f.FollowedAt)
	if err == nil {
		return domain.Change{Follow: f, Changed: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Change{}, fmt.Errorf("follow: %w", err)
	}
	// Already following: report the original time.
	err = platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT followed_at FROM social.follows WHERE follower_id = $1 AND target_type = $2 AND target_id = $3`,
		f.FollowerID, f.Target.Type, f.Target.ID).Scan(&f.FollowedAt)
	if err != nil {
		return domain.Change{}, fmt.Errorf("follow: read existing: %w", err)
	}
	return domain.Change{Follow: f}, nil
}

// Unfollow deletes the edge and decrements both counters in one statement.
func (r *Repository) Unfollow(ctx context.Context, follower uuid.UUID, target domain.Target, at time.Time) (domain.Change, error) {
	f := domain.Follow{FollowerID: follower, Target: target, FollowedAt: at}
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		WITH del AS (
			DELETE FROM social.follows WHERE follower_id = $1 AND target_type = $2 AND target_id = $3
			RETURNING 1
		), target AS (
			UPDATE social.counters SET followers = followers - 1
			WHERE subject_type = $2 AND subject_id = $3 AND EXISTS (SELECT 1 FROM del)
		), follower AS (
			UPDATE social.counters SET
				following_users   = following_users - ($2 = 'user')::int,
				following_artists = following_artists - ($2 = 'artist')::int
			WHERE subject_type = 'user' AND subject_id = $1 AND EXISTS (SELECT 1 FROM del)
		)
		SELECT 1 FROM del`, follower, target.Type, target.ID)
	if err != nil {
		return domain.Change{}, fmt.Errorf("unfollow: %w", err)
	}
	return domain.Change{Follow: f, Changed: tag.RowsAffected() == 1}, nil
}

func cursorArgs(after *domain.Cursor) (*time.Time, *uuid.UUID) {
	if after == nil {
		return nil, nil
	}
	return &after.At, &after.ID
}

// Followers returns who follows target, newest first after the cursor.
func (r *Repository) Followers(ctx context.Context, target domain.Target, after *domain.Cursor, limit int) ([]domain.Follow, error) {
	afterAt, afterID := cursorArgs(after)
	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT follower_id, followed_at FROM social.follows
		WHERE target_type = $1 AND target_id = $2
		  AND ($3::timestamptz IS NULL OR (followed_at, follower_id) < ($3, $4::uuid))
		ORDER BY followed_at DESC, follower_id DESC
		LIMIT $5`, target.Type, target.ID, afterAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("followers: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Follow, error) {
		f := domain.Follow{Target: target}
		err := row.Scan(&f.FollowerID, &f.FollowedAt)
		return f, err
	})
}

// Following returns the targets of one type follower follows, newest first.
func (r *Repository) Following(ctx context.Context, follower uuid.UUID, typ domain.TargetType, after *domain.Cursor, limit int) ([]domain.Follow, error) {
	afterAt, afterID := cursorArgs(after)
	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT target_id, followed_at FROM social.follows
		WHERE follower_id = $1 AND target_type = $2
		  AND ($3::timestamptz IS NULL OR (followed_at, target_id) < ($3, $4::uuid))
		ORDER BY followed_at DESC, target_id DESC
		LIMIT $5`, follower, typ, afterAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("following: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Follow, error) {
		f := domain.Follow{FollowerID: follower, Target: domain.Target{Type: typ}}
		err := row.Scan(&f.Target.ID, &f.FollowedAt)
		return f, err
	})
}

// Contains returns which ids follower follows.
func (r *Repository) Contains(ctx context.Context, follower uuid.UUID, typ domain.TargetType, ids []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := platformpg.Conn(ctx, r.pool).Query(ctx, `
		SELECT target_id FROM social.follows WHERE follower_id = $1 AND target_type = $2 AND target_id = ANY($3)`, follower, typ, ids)
	if err != nil {
		return nil, fmt.Errorf("contains: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// Counts reads the counters; a subject without a row has none.
func (r *Repository) Counts(ctx context.Context, subject domain.Target) (domain.Counts, error) {
	var c domain.Counts
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT followers, following_users, following_artists FROM social.counters
		WHERE subject_type = $1 AND subject_id = $2`, subject.Type, subject.ID).Scan(&c.Followers, &c.FollowingUsers, &c.FollowingArtists)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("counts: %w", err)
	}
	return c, nil
}
