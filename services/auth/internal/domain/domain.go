// Package domain holds Auth's model: accounts, credentials policy and
// sessions with rotating refresh tokens. No HTTP, SQL, Redis or JWT here.
package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Errors.
var (
	ErrEmailTaken         = errors.New("email is already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountNotFound    = errors.New("account not found")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionInactive    = errors.New("session is revoked or expired")
	// ErrRefreshReuse means a rotated refresh token was presented again:
	// it was probably stolen, so the whole session is revoked.
	ErrRefreshReuse     = errors.New("refresh token reuse detected")
	ErrInvalidRefresh   = errors.New("invalid refresh token")
	ErrTooManyAttempts  = errors.New("too many login attempts")
	ErrConcurrentUpdate = errors.New("session changed concurrently")
)

// ValidationError lists invalid fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Password policy (NIST 800-63B: length over composition rules).
const (
	MinPasswordLength = 10
	MaxPasswordLength = 128
)

// Role granted to every new account.
const RoleUser = "USER"

// Account is a set of credentials.
type Account struct {
	ID           uuid.UUID
	Email        string // normalized
	PasswordHash string // PHC string; never logged, never leaves the service
	Roles        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NormalizeEmail lower-cases and trims an address.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// ValidateCredentials checks the registration input.
func ValidateCredentials(email, password string) error {
	fields := map[string]string{}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email || len(email) > 254 {
		fields["email"] = "must be a valid email address"
	}
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLength:
		fields["password"] = "must be at least 10 characters"
	case n > MaxPasswordLength:
		fields["password"] = "must be at most 128 characters"
	case strings.TrimSpace(password) == "":
		fields["password"] = "must not be blank"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// NewAccount creates an account with the USER role.
func NewAccount(id uuid.UUID, email, passwordHash string, now time.Time) Account {
	now = now.UTC()
	return Account{ID: id, Email: email, PasswordHash: passwordHash, Roles: []string{RoleUser}, CreatedAt: now, UpdatedAt: now}
}

// Revoke reasons.
const (
	RevokeLogout     = "logout"
	RevokeExplicit   = "revoked"
	RevokeTokenReuse = "token_reuse"
)

// Session is one signed-in device. It holds only hashes of refresh tokens.
type Session struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	RefreshHash  []byte
	PreviousHash []byte // the hash rotated out last; presenting it again = reuse
	UserAgent    string
	CreatedAt    time.Time
	LastUsedAt   time.Time
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	RevokeReason string
}

// NewSession starts a session valid for ttl.
func NewSession(id, userID uuid.UUID, refreshHash []byte, userAgent string, now time.Time, ttl time.Duration) Session {
	now = now.UTC()
	if len(userAgent) > 256 {
		userAgent = userAgent[:256]
	}
	return Session{ID: id, UserID: userID, RefreshHash: refreshHash, UserAgent: userAgent, CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(ttl)}
}

// Active reports whether the session can still be used.
func (s Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// Rotate replaces the refresh token and slides the expiry.
func (s *Session) Rotate(newHash []byte, now time.Time, ttl time.Duration) error {
	if !s.Active(now) {
		return ErrSessionInactive
	}
	now = now.UTC()
	s.PreviousHash, s.RefreshHash = s.RefreshHash, newHash
	s.LastUsedAt = now
	s.ExpiresAt = now.Add(ttl)
	return nil
}

// Revoke ends the session. Revoking twice is a no-op.
func (s *Session) Revoke(reason string, now time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	t := now.UTC()
	s.RevokedAt, s.RevokeReason = &t, reason
	return true
}

// Event is a fact other services may react to.
type Event interface{ isAuthEvent() }

// UserRegistered is raised after an account is created.
type UserRegistered struct{ Account Account }

// SessionCreated is raised after sign-in.
type SessionCreated struct{ Session Session }

// SessionRevoked is raised after a session ends before expiry.
type SessionRevoked struct{ Session Session }

func (UserRegistered) isAuthEvent() {}
func (SessionCreated) isAuthEvent() {}
func (SessionRevoked) isAuthEvent() {}
