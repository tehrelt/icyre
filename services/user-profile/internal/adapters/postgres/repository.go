// Package postgres implements the profile repository with pgx.
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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by User Profile.
const Schema = "profile"

// OutboxTable holds profile.events messages until the relay sends them.
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
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool} }

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "profiles_username_key" {
		return domain.ErrUsernameTaken
	}
	return err
}

// CreateIfAbsent inserts the profile unless the user already has one.
func (r *Repository) CreateIfAbsent(ctx context.Context, p domain.Profile) (bool, error) {
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO profile.profiles (user_id, username, display_name, avatar_key, bio, country, language, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO NOTHING`,
		p.UserID, p.Username, p.DisplayName, p.AvatarKey, p.Bio, p.Country, p.Language, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return false, fmt.Errorf("insert profile: %w", mapErr(err))
	}
	return tag.RowsAffected() == 1, nil
}

// Get loads a profile.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (domain.Profile, error) {
	return r.get(ctx, id, false)
}

// GetForUpdate loads a profile with a row lock inside a transaction.
func (r *Repository) GetForUpdate(ctx context.Context, id uuid.UUID) (domain.Profile, error) {
	return r.get(ctx, id, true)
}

func (r *Repository) get(ctx context.Context, id uuid.UUID, forUpdate bool) (domain.Profile, error) {
	query := `
		SELECT user_id, username, display_name, avatar_key, bio, country, language, created_at, updated_at
		FROM profile.profiles WHERE user_id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var p domain.Profile
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, query, id).
		Scan(&p.UserID, &p.Username, &p.DisplayName, &p.AvatarKey, &p.Bio, &p.Country, &p.Language, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.Profile{}, fmt.Errorf("select profile: %w", err)
	}
	return p, nil
}

// Update stores the editable profile fields without touching avatar_key.
func (r *Repository) Update(ctx context.Context, p domain.Profile) error {
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		UPDATE profile.profiles
		SET username = $2, display_name = $3, bio = $4, country = $5, language = $6, updated_at = $7
		WHERE user_id = $1`,
		p.UserID, p.Username, p.DisplayName, p.Bio, p.Country, p.Language, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update profile: %w", mapErr(err))
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProfileNotFound
	}
	return nil
}

// SetAvatarKey updates only the avatar key and updated_at timestamp.
func (r *Repository) SetAvatarKey(ctx context.Context, userID uuid.UUID, avatarKey string, updatedAt time.Time) error {
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		UPDATE profile.profiles
		SET avatar_key = $2, updated_at = $3
		WHERE user_id = $1`,
		userID, avatarKey, updatedAt)
	if err != nil {
		return fmt.Errorf("set avatar key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProfileNotFound
	}
	return nil
}

// CreateAvatarUpload inserts a pending avatar upload session.
func (r *Repository) CreateAvatarUpload(ctx context.Context, u domain.AvatarUpload) error {
	_, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		INSERT INTO profile.avatar_uploads (id, user_id, content_type, size_bytes, sha256, object_key, status, failure_reason, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		u.ID, u.UserID, u.ContentType, u.SizeBytes, u.SHA256, u.Key, u.Status, u.FailureReason, u.CreatedAt, u.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create avatar upload: %w", err)
	}
	return nil
}

// GetAvatarUpload returns an avatar upload session or domain.ErrAvatarUploadNotFound.
func (r *Repository) GetAvatarUpload(ctx context.Context, id uuid.UUID) (domain.AvatarUpload, error) {
	var u domain.AvatarUpload
	err := platformpg.Conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, user_id, content_type, size_bytes, sha256, object_key, status, failure_reason, created_at, expires_at, completed_at
		FROM profile.avatar_uploads WHERE id = $1`, id).
		Scan(&u.ID, &u.UserID, &u.ContentType, &u.SizeBytes, &u.SHA256, &u.Key, &u.Status, &u.FailureReason, &u.CreatedAt, &u.ExpiresAt, &u.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AvatarUpload{}, domain.ErrAvatarUploadNotFound
	}
	if err != nil {
		return domain.AvatarUpload{}, fmt.Errorf("get avatar upload: %w", err)
	}
	return u, nil
}

// SettleAvatarUpload stores the final status only while the session is pending,
// so concurrent completes settle once.
func (r *Repository) SettleAvatarUpload(ctx context.Context, u domain.AvatarUpload) (bool, error) {
	tag, err := platformpg.Conn(ctx, r.pool).Exec(ctx, `
		UPDATE profile.avatar_uploads
		SET status = $2, failure_reason = $3, completed_at = $4
		WHERE id = $1 AND status = 'PENDING'`,
		u.ID, u.Status, u.FailureReason, u.CompletedAt)
	if err != nil {
		return false, fmt.Errorf("settle avatar upload: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
