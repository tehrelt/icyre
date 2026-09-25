// Package postgres implements Auth repositories with pgx.
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

	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Schema owned by Auth.
const Schema = "auth"

// Migrations returns the embedded migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// Accounts implements ports.AccountRepository.
type Accounts struct{ pool *pgxpool.Pool }

// NewAccounts returns an account repository.
func NewAccounts(pool *pgxpool.Pool) *Accounts { return &Accounts{pool} }

// Create inserts an account.
func (r *Accounts) Create(ctx context.Context, a domain.Account) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO auth.accounts (id, email, password_hash, roles, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ID, a.Email, a.PasswordHash, a.Roles, a.CreatedAt, a.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "accounts_email_key" {
		return domain.ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

const accountColumns = `id, email, password_hash, roles, created_at, updated_at`

func scanAccount(row pgx.Row) (domain.Account, error) {
	var a domain.Account
	err := row.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Roles, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("select account: %w", err)
	}
	return a, nil
}

// ByEmail loads an account by normalized email.
func (r *Accounts) ByEmail(ctx context.Context, email string) (domain.Account, error) {
	return scanAccount(r.pool.QueryRow(ctx, `SELECT `+accountColumns+` FROM auth.accounts WHERE email = $1`, email))
}

// ByID loads an account.
func (r *Accounts) ByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return scanAccount(r.pool.QueryRow(ctx, `SELECT `+accountColumns+` FROM auth.accounts WHERE id = $1`, id))
}

// Sessions implements ports.SessionRepository.
type Sessions struct{ pool *pgxpool.Pool }

// NewSessions returns a session repository.
func NewSessions(pool *pgxpool.Pool) *Sessions { return &Sessions{pool} }

const sessionColumns = `id, user_id, refresh_hash, previous_hash, user_agent, created_at, last_used_at, expires_at, revoked_at, COALESCE(revoke_reason, '')`

func scanSession(row pgx.Row) (domain.Session, error) {
	var s domain.Session
	err := row.Scan(&s.ID, &s.UserID, &s.RefreshHash, &s.PreviousHash, &s.UserAgent, &s.CreatedAt, &s.LastUsedAt, &s.ExpiresAt, &s.RevokedAt, &s.RevokeReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("select session: %w", err)
	}
	return s, nil
}

// Create inserts a session.
func (r *Sessions) Create(ctx context.Context, s domain.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_hash, user_agent, created_at, last_used_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, s.UserID, s.RefreshHash, s.UserAgent, s.CreatedAt, s.LastUsedAt, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// Get loads a session.
func (r *Sessions) Get(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return scanSession(r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM auth.sessions WHERE id = $1`, id))
}

// ByRefreshHash finds the session currently holding the token.
func (r *Sessions) ByRefreshHash(ctx context.Context, hash []byte) (domain.Session, error) {
	return scanSession(r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM auth.sessions WHERE refresh_hash = $1`, hash))
}

// ByPreviousHash finds the session that rotated the token out.
func (r *Sessions) ByPreviousHash(ctx context.Context, hash []byte) (domain.Session, error) {
	return scanSession(r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM auth.sessions WHERE previous_hash = $1`, hash))
}

// SaveRotation is a compare-and-swap on refresh_hash: of two concurrent
// refreshes with the same token, exactly one wins.
func (r *Sessions) SaveRotation(ctx context.Context, s domain.Session, oldHash []byte) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE auth.sessions
		SET refresh_hash = $2, previous_hash = $3, last_used_at = $4, expires_at = $5
		WHERE id = $1 AND refresh_hash = $3 AND revoked_at IS NULL`,
		s.ID, s.RefreshHash, oldHash, s.LastUsedAt, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("rotate session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

// SaveRevocation stores revoked_at/reason (first revocation wins).
func (r *Sessions) SaveRevocation(ctx context.Context, s domain.Session) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE auth.sessions SET revoked_at = $2, revoke_reason = $3 WHERE id = $1 AND revoked_at IS NULL`,
		s.ID, s.RevokedAt, s.RevokeReason)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// ListActive returns the user's live sessions, newest first.
func (r *Sessions) ListActive(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+sessionColumns+` FROM auth.sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
		ORDER BY created_at DESC LIMIT 100`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Session, error) { return scanSession(row) })
}
