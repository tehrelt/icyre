// Package application implements Auth use cases: register, login, refresh
// with rotation and reuse detection, logout and session management.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/auth/internal/domain"
	"github.com/tehrelt/icyre/services/auth/internal/ports"
)

// Deps are the collaborators of Service.
type Deps struct {
	Accounts   ports.AccountRepository
	Sessions   ports.SessionRepository
	Hasher     ports.PasswordHasher
	Tokens     ports.TokenIssuer
	Refresh    ports.RefreshTokens
	Revocation ports.Revocations
	Throttle   ports.LoginThrottle
	Publisher  ports.EventPublisher
	Log        *slog.Logger
	// SessionTTL is the sliding lifetime of a session (refresh token).
	SessionTTL time.Duration
	Now        func() time.Time
	NewID      func() (uuid.UUID, error)
}

// Service exposes the Auth use cases.
type Service struct {
	d         Deps
	dummyHash string
}

// New returns a Service.
func New(d Deps) (*Service, error) {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.NewID == nil {
		d.NewID = uuid.NewV7
	}
	if d.SessionTTL <= 0 {
		d.SessionTTL = 30 * 24 * time.Hour
	}
	// Verifying against a dummy hash for unknown emails keeps login timing
	// the same whether or not the account exists (no user enumeration).
	dummy, err := d.Hasher.Hash("dummy-password-for-timing")
	if err != nil {
		return nil, fmt.Errorf("dummy hash: %w", err)
	}
	return &Service{d: d, dummyHash: dummy}, nil
}

// Client describes the device signing in.
type Client struct {
	UserAgent string
}

// Result is what a successful sign-in or refresh returns.
type Result struct {
	Account      domain.Account
	SessionID    uuid.UUID
	Access       ports.AccessToken
	RefreshToken string
	RefreshUntil time.Time
}

// Register creates an account and signs it in.
func (s *Service) Register(ctx context.Context, email, password string, c Client) (Result, error) {
	email = domain.NormalizeEmail(email)
	if err := domain.ValidateCredentials(email, password); err != nil {
		return Result{}, err
	}
	hash, err := s.d.Hasher.Hash(password)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}
	id, err := s.d.NewID()
	if err != nil {
		return Result{}, err
	}
	acc := domain.NewAccount(id, email, hash, s.d.Now())
	if err := s.d.Accounts.Create(ctx, acc); err != nil {
		return Result{}, err
	}
	res, sess, err := s.startSession(ctx, acc, c)
	if err != nil {
		return Result{}, err
	}
	s.publish(ctx, domain.UserRegistered{Account: acc}, domain.SessionCreated{Session: sess})
	return res, nil
}

// Login verifies credentials and starts a session.
func (s *Service) Login(ctx context.Context, email, password string, c Client) (Result, error) {
	email = domain.NormalizeEmail(email)
	if ok, _, err := s.d.Throttle.Allow(ctx, email); err != nil {
		s.d.Log.WarnContext(ctx, "login throttle unavailable", "error", err)
	} else if !ok {
		return Result{}, domain.ErrTooManyAttempts
	}

	acc, err := s.d.Accounts.ByEmail(ctx, email)
	hash := acc.PasswordHash
	if errors.Is(err, domain.ErrAccountNotFound) {
		hash = s.dummyHash
	} else if err != nil {
		return Result{}, err
	}
	match, verr := s.d.Hasher.Verify(password, hash)
	if verr != nil {
		return Result{}, fmt.Errorf("verify password: %w", verr)
	}
	if err != nil || !match {
		if ferr := s.d.Throttle.Failed(ctx, email); ferr != nil {
			s.d.Log.WarnContext(ctx, "login throttle unavailable", "error", ferr)
		}
		return Result{}, domain.ErrInvalidCredentials
	}
	_ = s.d.Throttle.Reset(ctx, email)

	res, sess, err := s.startSession(ctx, acc, c)
	if err != nil {
		return Result{}, err
	}
	s.publish(ctx, domain.SessionCreated{Session: sess})
	return res, nil
}

// Refresh rotates the refresh token and issues a new access token.
// Presenting an already-rotated token revokes the session (theft signal).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Result, error) {
	if refreshToken == "" {
		return Result{}, domain.ErrInvalidRefresh
	}
	now := s.d.Now()
	oldHash := s.d.Refresh.Hash(refreshToken)

	sess, err := s.d.Sessions.ByRefreshHash(ctx, oldHash)
	if errors.Is(err, domain.ErrSessionNotFound) {
		if reused, rerr := s.d.Sessions.ByPreviousHash(ctx, oldHash); rerr == nil {
			// A stolen token must not outlive a failed revocation: surface it.
			if err := s.revoke(ctx, reused, domain.RevokeTokenReuse); err != nil {
				return Result{}, fmt.Errorf("revoke reused session: %w", err)
			}
			s.d.Log.WarnContext(ctx, "refresh token reuse: session revoked", "session_id", reused.ID.String())
			return Result{}, domain.ErrRefreshReuse
		}
		return Result{}, domain.ErrInvalidRefresh
	}
	if err != nil {
		return Result{}, err
	}
	if !sess.Active(now) {
		return Result{}, domain.ErrSessionInactive
	}

	token, newHash, err := s.d.Refresh.New()
	if err != nil {
		return Result{}, err
	}
	if err := sess.Rotate(newHash, now, s.d.SessionTTL); err != nil {
		return Result{}, err
	}
	if err := s.d.Sessions.SaveRotation(ctx, sess, oldHash); err != nil {
		return Result{}, err
	}
	acc, err := s.d.Accounts.ByID(ctx, sess.UserID)
	if err != nil {
		return Result{}, err
	}
	access, err := s.d.Tokens.Issue(acc, sess.ID, now)
	if err != nil {
		return Result{}, err
	}
	return Result{Account: acc, SessionID: sess.ID, Access: access, RefreshToken: token, RefreshUntil: sess.ExpiresAt}, nil
}

// Logout ends the session holding refreshToken. Unknown tokens are ignored:
// logout is idempotent.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	sess, err := s.d.Sessions.ByRefreshHash(ctx, s.d.Refresh.Hash(refreshToken))
	if errors.Is(err, domain.ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.revoke(ctx, sess, domain.RevokeLogout)
}

// Sessions lists the user's active sessions.
func (s *Service) Sessions(ctx context.Context, userID uuid.UUID) ([]domain.Session, error) {
	return s.d.Sessions.ListActive(ctx, userID, s.d.Now())
}

// RevokeSession ends one of the user's own sessions.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	sess, err := s.d.Sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.UserID != userID {
		// Do not reveal other users' sessions.
		return domain.ErrSessionNotFound
	}
	return s.revoke(ctx, sess, domain.RevokeExplicit)
}

func (s *Service) startSession(ctx context.Context, acc domain.Account, c Client) (Result, domain.Session, error) {
	now := s.d.Now()
	token, hash, err := s.d.Refresh.New()
	if err != nil {
		return Result{}, domain.Session{}, err
	}
	id, err := s.d.NewID()
	if err != nil {
		return Result{}, domain.Session{}, err
	}
	sess := domain.NewSession(id, acc.ID, hash, c.UserAgent, now, s.d.SessionTTL)
	if err := s.d.Sessions.Create(ctx, sess); err != nil {
		return Result{}, domain.Session{}, err
	}
	access, err := s.d.Tokens.Issue(acc, sess.ID, now)
	if err != nil {
		return Result{}, domain.Session{}, err
	}
	return Result{Account: acc, SessionID: sess.ID, Access: access, RefreshToken: token, RefreshUntil: sess.ExpiresAt}, sess, nil
}

// revoke persists the revocation and tells verifiers to stop accepting the
// session's outstanding access tokens.
func (s *Service) revoke(ctx context.Context, sess domain.Session, reason string) error {
	if !sess.Revoke(reason, s.d.Now()) {
		return nil
	}
	if err := s.d.Sessions.SaveRevocation(ctx, sess); err != nil {
		return err
	}
	if err := s.d.Revocation.Revoke(ctx, sess.ID.String(), s.d.Tokens.TTL()); err != nil {
		// Access tokens then live until expiry (minutes); refresh is already dead.
		s.d.Log.WarnContext(ctx, "revocation cache write failed", "error", err)
	}
	s.publish(ctx, domain.SessionRevoked{Session: sess})
	return nil
}

func (s *Service) publish(ctx context.Context, events ...domain.Event) {
	if err := s.d.Publisher.Publish(ctx, events...); err != nil {
		s.d.Log.ErrorContext(ctx, "publish auth events failed", "error", err, "count", len(events))
	}
}
