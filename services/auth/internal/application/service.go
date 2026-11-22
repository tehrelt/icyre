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
	Accounts           ports.AccountRepository
	Sessions           ports.SessionRepository
	Receipts           ports.ModerationReceiptRepository
	PendingRevocations ports.PendingRevocationRepository
	Hasher             ports.PasswordHasher
	Tokens             ports.TokenIssuer
	Refresh            ports.RefreshTokens
	Revocation         ports.Revocations
	Throttle           ports.LoginThrottle
	Publisher          ports.EventPublisher
	// Tx is the unit of work (writes + outbox events); nil runs without one.
	Tx  ports.Transactor
	Log *slog.Logger
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
	if d.Tx == nil {
		d.Tx = noTx{}
	}
	if d.Log == nil {
		d.Log = slog.Default()
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
	// Account, session and user.registered commit together: User Profile
	// learns about every account, and only about real ones.
	var res Result
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Accounts.Create(ctx, acc); err != nil {
			return err
		}
		var sess domain.Session
		if res, sess, err = s.startSession(ctx, acc, c); err != nil {
			return err
		}
		return s.record(ctx, domain.UserRegistered{Account: acc}, domain.SessionCreated{Session: sess})
	})
	if err != nil {
		return Result{}, err
	}
	s.d.Log.InfoContext(ctx, "auth audit", "audit", "auth", "event", "register", "user_id", acc.ID.String(), "session_id", res.SessionID.String())
	return res, nil
}

// Login verifies credentials and starts a session.
func (s *Service) Login(ctx context.Context, email, password string, c Client) (Result, error) {
	email = domain.NormalizeEmail(email)
	if ok, _, err := s.d.Throttle.Allow(ctx, email); err != nil {
		s.d.Log.WarnContext(ctx, "login throttle unavailable", "error", err)
	} else if !ok {
		s.d.Log.WarnContext(ctx, "auth audit", "audit", "auth", "event", "login_throttled")
		return Result{}, domain.ErrTooManyAttempts
	}

	acc, err := s.verifyLoginCredentials(ctx, email, password)
	if err != nil {
		return Result{}, err
	}
	_ = s.d.Throttle.Reset(ctx, email)

	var res Result
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		locked, err := s.d.Accounts.ByIDForUpdate(ctx, acc.ID)
		if err != nil {
			return err
		}
		if locked.Blocked {
			return domain.ErrAccountBlocked
		}
		var sess domain.Session
		if res, sess, err = s.startSession(ctx, locked, c); err != nil {
			return err
		}
		return s.record(ctx, domain.SessionCreated{Session: sess})
	})
	if errors.Is(err, domain.ErrAccountBlocked) {
		s.d.Log.WarnContext(ctx, "auth audit", "audit", "auth", "event", "login_blocked", "user_id", acc.ID.String())
		return Result{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return Result{}, err
	}
	s.d.Log.InfoContext(ctx, "auth audit", "audit", "auth", "event", "login_succeeded", "user_id", acc.ID.String(), "session_id", res.SessionID.String())
	return res, nil
}

func (s *Service) verifyLoginCredentials(ctx context.Context, email, password string) (domain.Account, error) {
	acc, err := s.d.Accounts.ByEmail(ctx, email)
	hash := acc.PasswordHash
	if errors.Is(err, domain.ErrAccountNotFound) {
		hash = s.dummyHash
	} else if err != nil {
		return domain.Account{}, err
	}
	match, verr := s.d.Hasher.Verify(password, hash)
	if verr != nil {
		return domain.Account{}, fmt.Errorf("verify password: %w", verr)
	}
	if err != nil || !match {
		if ferr := s.d.Throttle.Failed(ctx, email); ferr != nil {
			s.d.Log.WarnContext(ctx, "login throttle unavailable", "error", ferr)
		}
		s.d.Log.WarnContext(ctx, "auth audit", "audit", "auth", "event", "login_failed")
		return domain.Account{}, domain.ErrInvalidCredentials
	}
	return acc, nil
}

// Refresh rotates the refresh token and issues a new access token.
// Presenting an already-rotated token revokes the session (theft signal).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Result, error) {
	if refreshToken == "" {
		return Result{}, domain.ErrInvalidRefresh
	}
	now := s.d.Now()
	oldHash := s.d.Refresh.Hash(refreshToken)

	sess, err := s.lookupRefreshSession(ctx, oldHash)
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

	var res Result
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		acc, err := s.d.Accounts.ByIDForUpdate(ctx, sess.UserID)
		if err != nil {
			return err
		}
		if acc.Blocked {
			return domain.ErrSessionInactive
		}
		current, err := s.d.Sessions.Get(ctx, sess.ID)
		if err != nil {
			return err
		}
		if !current.Active(now) {
			return domain.ErrSessionInactive
		}
		if err := current.Rotate(newHash, now, s.d.SessionTTL); err != nil {
			return err
		}
		if err := s.d.Sessions.SaveRotation(ctx, current, oldHash); err != nil {
			return err
		}
		access, err := s.d.Tokens.Issue(acc, current.ID, now)
		if err != nil {
			return err
		}
		res = Result{Account: acc, SessionID: current.ID, Access: access, RefreshToken: token, RefreshUntil: current.ExpiresAt}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

func (s *Service) lookupRefreshSession(ctx context.Context, oldHash []byte) (domain.Session, error) {
	sess, err := s.d.Sessions.ByRefreshHash(ctx, oldHash)
	if errors.Is(err, domain.ErrSessionNotFound) {
		if reused, rerr := s.d.Sessions.ByPreviousHash(ctx, oldHash); rerr == nil {
			// A stolen token must not outlive a failed revocation: surface it.
			if err := s.revoke(ctx, reused, domain.RevokeTokenReuse); err != nil {
				return domain.Session{}, fmt.Errorf("revoke reused session: %w", err)
			}
			s.d.Log.WarnContext(ctx, "refresh token reuse: session revoked", "audit", "auth", "event", "refresh_reuse_revoked", "user_id", reused.UserID.String(), "session_id", reused.ID.String())
			return domain.Session{}, domain.ErrRefreshReuse
		}
		return domain.Session{}, domain.ErrInvalidRefresh
	}
	if err != nil {
		return domain.Session{}, err
	}
	return sess, nil
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

// ListModerationUsers returns accounts ordered by ID for moderation listing.
func (s *Service) ListModerationUsers(ctx context.Context, afterID *uuid.UUID, limit int, blocked *bool) ([]domain.Account, error) {
	return s.d.Accounts.ListAccounts(ctx, afterID, limit, blocked)
}

// GetModerationUser returns a single account for moderation inspection.
func (s *Service) GetModerationUser(ctx context.Context, userID uuid.UUID) (domain.Account, error) {
	return s.d.Accounts.ByID(ctx, userID)
}

// ModerateBlock blocks or unblocks an account idempotently by operationID.
func (s *Service) ModerateBlock(ctx context.Context, operationID, userID uuid.UUID, blocked bool) (domain.ModerationReceipt, error) {
	if s.d.Receipts == nil {
		return domain.ModerationReceipt{}, errors.New("moderation receipt repository is not configured")
	}
	var (
		rec        domain.ModerationReceipt
		revokedIDs []uuid.UUID
		replayed   bool
	)
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		rec, revokedIDs, replayed, err = s.applyModerationBlock(ctx, operationID, userID, blocked)
		return err
	})
	if err != nil {
		return domain.ModerationReceipt{}, err
	}
	if replayed {
		return rec, nil
	}
	if rec.Changed {
		s.d.Log.InfoContext(ctx, "auth audit", "audit", "auth", "event", "account_moderation", "user_id", userID.String(), "operation_id", operationID.String(), "blocked", blocked, "revoked_sessions", rec.RevokedSessions)
	}
	rec.RevocationsPending = s.revokeSessionsInCache(ctx, userID, revokedIDs)
	return rec, nil
}

func (s *Service) applyModerationBlock(ctx context.Context, operationID, userID uuid.UUID, blocked bool) (domain.ModerationReceipt, []uuid.UUID, bool, error) {
	if existing, err := s.d.Receipts.GetModerationReceipt(ctx, operationID); err == nil {
		rec, err := matchModerationReceipt(existing, userID, blocked)
		return rec, nil, true, err
	} else if !errors.Is(err, domain.ErrReceiptNotFound) {
		return domain.ModerationReceipt{}, nil, false, err
	}

	acc, err := s.d.Accounts.ByIDForUpdate(ctx, userID)
	if err != nil {
		return domain.ModerationReceipt{}, nil, false, err
	}
	if blocked && acc.HasRole(domain.RoleAdmin) {
		return domain.ModerationReceipt{}, nil, false, domain.ErrCannotBlockAdmin
	}

	now := s.d.Now().UTC()
	if acc.Blocked == blocked {
		rec := domain.ModerationReceipt{OperationID: operationID, UserID: userID, Blocked: blocked, Changed: false, CreatedAt: now}
		if err := s.d.Receipts.SaveModerationReceipt(ctx, rec); err != nil {
			return domain.ModerationReceipt{}, nil, false, err
		}
		return rec, nil, false, nil
	}

	return s.commitModerationChange(ctx, operationID, userID, blocked, now)
}

func (s *Service) commitModerationChange(ctx context.Context, operationID, userID uuid.UUID, blocked bool, now time.Time) (domain.ModerationReceipt, []uuid.UUID, bool, error) {
	var blockedAt *time.Time
	if blocked {
		blockedAt = &now
	}
	if err := s.d.Accounts.SetBlocked(ctx, userID, blocked, blockedAt); err != nil {
		return domain.ModerationReceipt{}, nil, false, err
	}
	var revokedIDs []uuid.UUID
	if blocked {
		var err error
		revokedIDs, err = s.revokeAllUserSessions(ctx, userID, now)
		if err != nil {
			return domain.ModerationReceipt{}, nil, false, err
		}
	}
	rec := domain.ModerationReceipt{
		OperationID:     operationID,
		UserID:          userID,
		Blocked:         blocked,
		Changed:         true,
		RevokedSessions: len(revokedIDs),
		CreatedAt:       now,
	}
	if err := s.d.Receipts.SaveModerationReceipt(ctx, rec); err != nil {
		return domain.ModerationReceipt{}, nil, false, err
	}
	return rec, revokedIDs, false, nil
}

func (s *Service) revokeAllUserSessions(ctx context.Context, userID uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	revokedIDs, err := s.d.Sessions.RevokeAllUserSessions(ctx, userID, domain.RevokeAccountBlocked, now)
	if err != nil {
		return nil, err
	}
	if len(revokedIDs) == 0 {
		return nil, nil
	}
	revokedAt := now.UTC()
	events := make([]domain.Event, 0, len(revokedIDs))
	for _, sid := range revokedIDs {
		events = append(events, domain.SessionRevoked{
			Session: domain.Session{
				ID:           sid,
				UserID:       userID,
				RevokedAt:    &revokedAt,
				RevokeReason: domain.RevokeAccountBlocked,
			},
		})
	}
	if err := s.record(ctx, events...); err != nil {
		return nil, err
	}
	if s.d.PendingRevocations != nil {
		ttl := s.d.Tokens.TTL() + time.Minute
		if err := s.d.PendingRevocations.SavePendingRevocations(ctx, revokedIDs, revokedAt.Add(ttl)); err != nil {
			return nil, err
		}
	}
	return revokedIDs, nil
}

func (s *Service) revokeSessionsInCache(ctx context.Context, userID uuid.UUID, sessionIDs []uuid.UUID) bool {
	if len(sessionIDs) == 0 {
		return false
	}
	ttl := s.d.Tokens.TTL() + time.Minute
	pending := false
	for _, sid := range sessionIDs {
		if err := s.d.Revocation.Revoke(ctx, sid.String(), ttl); err != nil {
			pending = true
			s.d.Log.WarnContext(ctx, "revocation cache write failed", "error", err, "user_id", userID.String(), "session_id", sid.String())
			continue
		}
		if s.d.PendingRevocations != nil {
			if err := s.d.PendingRevocations.DeletePendingRevocation(ctx, sid); err != nil {
				s.d.Log.WarnContext(ctx, "delete pending revocation failed", "error", err, "session_id", sid.String())
			}
		}
	}
	return pending
}

// FlushPendingRevocations drains durable pending revocations into the revocation cache.
func (s *Service) FlushPendingRevocations(ctx context.Context) error {
	if s.d.PendingRevocations == nil {
		return nil
	}
	items, err := s.d.PendingRevocations.ListPendingRevocations(ctx, 100)
	if err != nil {
		return err
	}
	now := s.d.Now().UTC()
	var errs []error
	for _, item := range items {
		if ttl := item.ExpiresAt.Sub(now); ttl > 0 {
			if err := s.d.Revocation.Revoke(ctx, item.SessionID.String(), ttl); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		if err := s.d.PendingRevocations.DeletePendingRevocation(ctx, item.SessionID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func matchModerationReceipt(r domain.ModerationReceipt, userID uuid.UUID, blocked bool) (domain.ModerationReceipt, error) {
	if r.UserID != userID || r.Blocked != blocked {
		return domain.ModerationReceipt{}, domain.ErrOperationConflict
	}
	return r, nil
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
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Sessions.SaveRevocation(ctx, sess); err != nil {
			return err
		}
		return s.record(ctx, domain.SessionRevoked{Session: sess})
	})
	if err != nil {
		return err
	}
	s.d.Log.InfoContext(ctx, "auth audit", "audit", "auth", "event", "session_revoked", "user_id", sess.UserID.String(), "session_id", sess.ID.String(), "reason", reason)
	if err := s.d.Revocation.Revoke(ctx, sess.ID.String(), s.d.Tokens.TTL()); err != nil {
		// Access tokens then live until expiry (minutes); refresh is already dead.
		s.d.Log.WarnContext(ctx, "revocation cache write failed", "error", err)
	}
	return nil
}

// record writes events in the current unit of work (transactional outbox):
// they are published if and only if the change commits.
func (s *Service) record(ctx context.Context, events ...domain.Event) error {
	if err := s.d.Publisher.Publish(ctx, events...); err != nil {
		return fmt.Errorf("record auth events: %w", err)
	}
	return nil
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
