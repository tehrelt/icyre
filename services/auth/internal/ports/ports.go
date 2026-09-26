// Package ports declares what Auth's application layer needs.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

// AccountRepository persists accounts. Create returns domain.ErrEmailTaken on
// a duplicate email; lookups return domain.ErrAccountNotFound.
type AccountRepository interface {
	Create(ctx context.Context, a domain.Account) error
	ByEmail(ctx context.Context, email string) (domain.Account, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
}

// SessionRepository persists sessions. Lookups return domain.ErrSessionNotFound.
type SessionRepository interface {
	Create(ctx context.Context, s domain.Session) error
	Get(ctx context.Context, id uuid.UUID) (domain.Session, error)
	ByRefreshHash(ctx context.Context, hash []byte) (domain.Session, error)
	ByPreviousHash(ctx context.Context, hash []byte) (domain.Session, error)
	// SaveRotation stores a rotated session only if its refresh hash is still
	// oldHash (compare-and-swap), else domain.ErrConcurrentUpdate.
	SaveRotation(ctx context.Context, s domain.Session, oldHash []byte) error
	SaveRevocation(ctx context.Context, s domain.Session) error
	ListActive(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.Session, error)
}

// PasswordHasher hashes and verifies passwords (Argon2id).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}

// AccessToken is a signed, short-lived bearer token.
type AccessToken struct {
	Token     string
	ExpiresAt time.Time
}

// TokenIssuer signs access tokens.
type TokenIssuer interface {
	Issue(a domain.Account, sessionID uuid.UUID, now time.Time) (AccessToken, error)
	// TTL is the access token lifetime (used to size revocation entries).
	TTL() time.Duration
}

// RefreshTokens generates opaque refresh tokens and their storage hashes.
type RefreshTokens interface {
	New() (token string, hash []byte, err error)
	Hash(token string) []byte
}

// Revocations tells token verifiers that a session ended early.
type Revocations interface {
	Revoke(ctx context.Context, sessionID string, ttl time.Duration) error
}

// LoginThrottle limits failed sign-in attempts per account.
type LoginThrottle interface {
	// Allow reports whether another attempt is allowed and, if not, how long to wait.
	Allow(ctx context.Context, email string) (bool, time.Duration, error)
	Failed(ctx context.Context, email string) error
	Reset(ctx context.Context, email string) error
}

// EventPublisher delivers domain events.
type EventPublisher interface {
	Publish(ctx context.Context, events ...domain.Event) error
}

// Transactor runs fn as one unit of work: repository writes and published
// events inside it commit or roll back together (transactional outbox).
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
