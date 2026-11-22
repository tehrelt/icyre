package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/auth/internal/domain"
	"github.com/tehrelt/icyre/services/auth/internal/ports"
)

// In-memory fakes.

type memAccounts map[uuid.UUID]domain.Account

func (m memAccounts) Create(_ context.Context, a domain.Account) error {
	for _, x := range m {
		if x.Email == a.Email {
			return domain.ErrEmailTaken
		}
	}
	m[a.ID] = a
	return nil
}
func (m memAccounts) ByEmail(_ context.Context, email string) (domain.Account, error) {
	for _, x := range m {
		if x.Email == email {
			return x, nil
		}
	}
	return domain.Account{}, domain.ErrAccountNotFound
}
func (m memAccounts) ByID(_ context.Context, id uuid.UUID) (domain.Account, error) {
	a, ok := m[id]
	if !ok {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	return a, nil
}
func (m memAccounts) ByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return m.ByID(ctx, id)
}
func (m memAccounts) ByEmailForUpdate(ctx context.Context, email string) (domain.Account, error) {
	return m.ByEmail(ctx, email)
}
func (m memAccounts) ListAccounts(_ context.Context, afterID *uuid.UUID, limit int, blocked *bool) ([]domain.Account, error) {
	if limit <= 0 {
		limit = 50
	}
	out := make([]domain.Account, 0, len(m))
	for _, a := range m {
		if afterID != nil && *afterID != uuid.Nil && bytes.Compare(a.ID[:], (*afterID)[:]) <= 0 {
			continue
		}
		if blocked != nil && a.Blocked != *blocked {
			continue
		}
		out = append(out, a)
	}
	slices.SortFunc(out, func(a, b domain.Account) int {
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m memAccounts) SetBlocked(_ context.Context, userID uuid.UUID, blocked bool, at *time.Time) error {
	a, ok := m[userID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	ts := time.Now().UTC()
	if at != nil {
		ts = at.UTC()
	}
	a.Blocked = blocked
	if blocked {
		a.BlockedAt = &ts
	} else {
		a.BlockedAt = nil
	}
	a.UpdatedAt = ts
	m[userID] = a
	return nil
}

type memSessions map[uuid.UUID]domain.Session

func (m memSessions) Create(_ context.Context, s domain.Session) error { m[s.ID] = s; return nil }
func (m memSessions) Get(_ context.Context, id uuid.UUID) (domain.Session, error) {
	s, ok := m[id]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	return s, nil
}
func (m memSessions) find(match func(domain.Session) bool) (domain.Session, error) {
	for _, s := range m {
		if match(s) {
			return s, nil
		}
	}
	return domain.Session{}, domain.ErrSessionNotFound
}
func (m memSessions) ByRefreshHash(_ context.Context, h []byte) (domain.Session, error) {
	return m.find(func(s domain.Session) bool { return bytes.Equal(s.RefreshHash, h) })
}
func (m memSessions) ByPreviousHash(_ context.Context, h []byte) (domain.Session, error) {
	return m.find(func(s domain.Session) bool { return s.PreviousHash != nil && bytes.Equal(s.PreviousHash, h) })
}
func (m memSessions) SaveRotation(_ context.Context, s domain.Session, old []byte) error {
	if !bytes.Equal(m[s.ID].RefreshHash, old) {
		return domain.ErrConcurrentUpdate
	}
	m[s.ID] = s
	return nil
}
func (m memSessions) SaveRevocation(_ context.Context, s domain.Session) error {
	m[s.ID] = s
	return nil
}
func (m memSessions) RevokeAllUserSessions(_ context.Context, userID uuid.UUID, reason string, at time.Time) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for id, s := range m {
		if s.UserID == userID && s.RevokedAt == nil {
			s.Revoke(reason, at)
			m[id] = s
			ids = append(ids, id)
		}
	}
	return ids, nil
}
func (m memSessions) ListActive(_ context.Context, uid uuid.UUID, now time.Time) ([]domain.Session, error) {
	var out []domain.Session
	for _, s := range m {
		if s.UserID == uid && s.Active(now) {
			out = append(out, s)
		}
	}
	return out, nil
}

// fakeHasher is reversible on purpose: tests only need equality semantics.
type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error)      { return "hash:" + p, nil }
func (fakeHasher) Verify(p, enc string) (bool, error) { return enc == "hash:"+p, nil }

type fakeTokens struct{}

func (fakeTokens) Issue(a domain.Account, sid uuid.UUID, now time.Time) (ports.AccessToken, error) {
	return ports.AccessToken{Token: "access:" + a.ID.String() + ":" + sid.String(), ExpiresAt: now.Add(15 * time.Minute)}, nil
}
func (fakeTokens) TTL() time.Duration { return 15 * time.Minute }

type seqRefresh struct{ n int }

func (r *seqRefresh) New() (string, []byte, error) {
	r.n++
	t := fmt.Sprintf("refresh-%d", r.n)
	return t, r.Hash(t), nil
}
func (*seqRefresh) Hash(t string) []byte { h := sha256.Sum256([]byte(t)); return h[:] }

type memRevocations map[string]time.Duration

func (m memRevocations) Revoke(_ context.Context, sid string, ttl time.Duration) error {
	m[sid] = ttl
	return nil
}

type failingRevocations struct{ err error }

func (f failingRevocations) Revoke(context.Context, string, time.Duration) error { return f.err }

type memReceipts map[uuid.UUID]domain.ModerationReceipt

func (m memReceipts) GetModerationReceipt(_ context.Context, opID uuid.UUID) (domain.ModerationReceipt, error) {
	r, ok := m[opID]
	if !ok {
		return domain.ModerationReceipt{}, domain.ErrReceiptNotFound
	}
	return r, nil
}
func (m memReceipts) SaveModerationReceipt(_ context.Context, r domain.ModerationReceipt) error {
	m[r.OperationID] = r
	return nil
}

type memPendingRevocations map[uuid.UUID]domain.PendingRevocation

func (m memPendingRevocations) SavePendingRevocations(_ context.Context, sids []uuid.UUID, expiresAt time.Time) error {
	for _, sid := range sids {
		m[sid] = domain.PendingRevocation{SessionID: sid, ExpiresAt: expiresAt}
	}
	return nil
}
func (m memPendingRevocations) DeletePendingRevocation(_ context.Context, sid uuid.UUID) error {
	delete(m, sid)
	return nil
}
func (m memPendingRevocations) ListPendingRevocations(_ context.Context, limit int) ([]domain.PendingRevocation, error) {
	out := make([]domain.PendingRevocation, 0, len(m))
	for _, p := range m {
		out = append(out, p)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type memThrottle struct{ failures map[string]int }

func (t *memThrottle) Allow(_ context.Context, e string) (bool, time.Duration, error) {
	return t.failures[e] < 3, time.Minute, nil
}
func (t *memThrottle) Failed(_ context.Context, e string) error { t.failures[e]++; return nil }
func (t *memThrottle) Reset(_ context.Context, e string) error  { delete(t.failures, e); return nil }

type recorder struct{ events []domain.Event }

func (r *recorder) Publish(_ context.Context, e ...domain.Event) error {
	r.events = append(r.events, e...)
	return nil
}

type fixture struct {
	svc      *Service
	accounts memAccounts
	sessions memSessions
	receipts memReceipts
	pending  memPendingRevocations
	revoked  memRevocations
	pub      *recorder
	now      time.Time
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{
		accounts: memAccounts{},
		sessions: memSessions{},
		receipts: memReceipts{},
		pending:  memPendingRevocations{},
		revoked:  memRevocations{},
		pub:      &recorder{},
		now:      time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
	}
	svc, err := New(Deps{
		Accounts: f.accounts, Sessions: f.sessions, Receipts: f.receipts, PendingRevocations: f.pending,
		Hasher: fakeHasher{}, Tokens: fakeTokens{}, Refresh: &seqRefresh{},
		Revocation: f.revoked, Throttle: &memThrottle{failures: map[string]int{}}, Publisher: f.pub,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionTTL: 24 * time.Hour,
		Now: func() time.Time { return f.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return f
}

const pw = "correct horse battery"

func TestRegisterNormalisesAndPublishes(t *testing.T) {
	f := newFixture(t)
	res, err := f.svc.Register(context.Background(), "  Rin@Example.COM ", pw, Client{UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Account.Email != "rin@example.com" || res.RefreshToken == "" || !strings.HasPrefix(res.Access.Token, "access:") {
		t.Fatalf("result = %+v", res)
	}
	if res.Account.PasswordHash == pw {
		t.Fatal("password stored in plaintext")
	}
	if len(f.pub.events) != 2 {
		t.Fatalf("events = %d", len(f.pub.events))
	}
	if _, ok := f.pub.events[0].(domain.UserRegistered); !ok {
		t.Fatalf("first event %T", f.pub.events[0])
	}

	if _, err := f.svc.Register(context.Background(), "rin@example.com", pw, Client{}); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	var ve *domain.ValidationError
	if _, err := f.svc.Register(context.Background(), "not-an-email", "short", Client{}); !errors.As(err, &ve) || ve.Fields["email"] == "" || ve.Fields["password"] == "" {
		t.Fatalf("validation: %v", err)
	}
}

type failingPub struct{}

func (failingPub) Publish(context.Context, ...domain.Event) error {
	return errors.New("outbox insert failed")
}

type rollbackTx struct{ rolledBack int }

func (r *rollbackTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	err := fn(ctx)
	if err != nil {
		r.rolledBack++
	}
	return err
}

func TestRegisterFailsWithoutItsEvent(t *testing.T) {
	tx := &rollbackTx{}
	svc, err := New(Deps{
		Accounts: memAccounts{}, Sessions: memSessions{}, Hasher: fakeHasher{}, Tokens: fakeTokens{}, Refresh: &seqRefresh{},
		Revocation: memRevocations{}, Throttle: &memThrottle{failures: map[string]int{}}, Publisher: failingPub{}, Tx: tx,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	// No account may exist that User Profile never hears about.
	if _, err := svc.Register(context.Background(), "rin@example.com", pw, Client{}); err == nil || tx.rolledBack != 1 {
		t.Fatalf("register must fail and roll back: %v, rollbacks %d", err, tx.rolledBack)
	}
}

func TestLoginAndThrottle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Register(ctx, "rin@example.com", pw, Client{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "RIN@example.com", pw, Client{}); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := f.svc.Login(ctx, "nobody@example.com", pw, Client{}); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("unknown email must look like a bad password: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := f.svc.Login(ctx, "rin@example.com", "wrong password!", Client{}); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := f.svc.Login(ctx, "rin@example.com", pw, Client{}); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("expected throttling, got %v", err)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first, _ := f.svc.Register(ctx, "rin@example.com", pw, Client{})

	f.now = f.now.Add(time.Hour)
	second, err := f.svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken || second.SessionID != first.SessionID {
		t.Fatalf("rotation failed: %+v", second)
	}
	if !second.RefreshUntil.Equal(f.now.Add(24 * time.Hour)) {
		t.Fatalf("expiry must slide: %v", second.RefreshUntil)
	}

	// The old token comes back: someone else has it. Kill the session.
	if _, err := f.svc.Refresh(ctx, first.RefreshToken); !errors.Is(err, domain.ErrRefreshReuse) {
		t.Fatalf("expected reuse detection, got %v", err)
	}
	if _, ok := f.revoked[first.SessionID.String()]; !ok {
		t.Fatal("reused session not pushed to the revocation list")
	}
	if _, err := f.svc.Refresh(ctx, second.RefreshToken); !errors.Is(err, domain.ErrSessionInactive) {
		t.Fatalf("legit token must die with the session: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, "garbage"); !errors.Is(err, domain.ErrInvalidRefresh) {
		t.Fatalf("garbage: %v", err)
	}
}

func TestRefreshAfterExpiry(t *testing.T) {
	f := newFixture(t)
	res, _ := f.svc.Register(context.Background(), "rin@example.com", pw, Client{})
	f.now = f.now.Add(25 * time.Hour)
	if _, err := f.svc.Refresh(context.Background(), res.RefreshToken); !errors.Is(err, domain.ErrSessionInactive) {
		t.Fatalf("expired session refreshed: %v", err)
	}
}

func TestLogoutAndSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, _ := f.svc.Register(ctx, "rin@example.com", pw, Client{UserAgent: "phone"})
	b, _ := f.svc.Login(ctx, "rin@example.com", pw, Client{UserAgent: "laptop"})

	list, _ := f.svc.Sessions(ctx, a.Account.ID)
	if len(list) != 2 {
		t.Fatalf("sessions = %d", len(list))
	}
	if err := f.svc.Logout(ctx, a.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Logout(ctx, a.RefreshToken); err != nil {
		t.Fatalf("logout must be idempotent: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, a.RefreshToken); err == nil {
		t.Fatal("refresh after logout succeeded")
	}
	if ttl := f.revoked[a.SessionID.String()]; ttl != 15*time.Minute {
		t.Fatalf("revocation ttl = %v", ttl)
	}

	stranger := uuid.New()
	if err := f.svc.RevokeSession(ctx, stranger, b.SessionID); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("revoking another user's session: %v", err)
	}
	if err := f.svc.RevokeSession(ctx, b.Account.ID, b.SessionID); err != nil {
		t.Fatal(err)
	}
	list, _ = f.svc.Sessions(ctx, a.Account.ID)
	if len(list) != 0 {
		t.Fatalf("active sessions left: %d", len(list))
	}
}

func TestAuditLoggingAndNilLoggerDefault(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	accounts := memAccounts{}
	sessions := memSessions{}
	revoked := memRevocations{}
	pub := &recorder{}
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	svc, err := New(Deps{
		Accounts: accounts, Sessions: sessions, Hasher: fakeHasher{}, Tokens: fakeTokens{}, Refresh: &seqRefresh{},
		Revocation: revoked, Throttle: &memThrottle{failures: map[string]int{}}, Publisher: pub,
		Log: logger, SessionTTL: 24 * time.Hour, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.d.Log != logger {
		t.Fatal("expected provided logger")
	}

	nilLogSvc, err := New(Deps{
		Accounts: memAccounts{}, Sessions: memSessions{}, Hasher: fakeHasher{}, Tokens: fakeTokens{}, Refresh: &seqRefresh{},
		Revocation: memRevocations{}, Throttle: &memThrottle{failures: map[string]int{}}, Publisher: &recorder{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if nilLogSvc.d.Log == nil {
		t.Fatal("expected nil logger to default to slog.Default()")
	}

	ctx := context.Background()
	email := "audit-secret@example.com"
	reg, err := svc.Register(ctx, email, pw, Client{})
	if err != nil {
		t.Fatal(err)
	}
	loginRes, err := svc.Login(ctx, email, pw, Client{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		_, _ = svc.Login(ctx, email, "wrong password!", Client{})
	}
	_, _ = svc.Login(ctx, email, pw, Client{})

	rotated, err := svc.Refresh(ctx, reg.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = svc.Refresh(ctx, reg.RefreshToken)
	_ = svc.Logout(ctx, loginRes.RefreshToken)

	logs := buf.String()
	for _, want := range []string{
		`"audit":"auth"`,
		`"event":"register"`,
		`"event":"login_succeeded"`,
		`"event":"login_failed"`,
		`"event":"login_throttled"`,
		`"event":"refresh_reuse_revoked"`,
		`"event":"session_revoked"`,
		`"user_id":"` + reg.Account.ID.String() + `"`,
		`"session_id":"` + rotated.SessionID.String() + `"`,
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("missing %s in logs:\n%s", want, logs)
		}
	}
	for _, forbidden := range []string{email, pw, reg.RefreshToken, reg.Access.Token, loginRes.RefreshToken, loginRes.Access.Token} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("sensitive value %q leaked in logs:\n%s", forbidden, logs)
		}
	}
}

func TestBlockedLoginAndRefreshRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	reg, err := f.svc.Register(ctx, "rin@example.com", pw, Client{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ModerateBlock(ctx, uuid.New(), reg.Account.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "rin@example.com", pw, Client{}); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("blocked login must return ErrInvalidCredentials, got %v", err)
	}
	if _, err := f.svc.Refresh(ctx, reg.RefreshToken); !errors.Is(err, domain.ErrSessionInactive) {
		t.Fatalf("blocked refresh must return ErrSessionInactive, got %v", err)
	}
	// Even if a session remained active in storage, blocked account cannot refresh.
	s := domain.NewSession(uuid.New(), reg.Account.ID, (&seqRefresh{}).Hash("manual-refresh"), "ua", f.now, time.Hour)
	f.sessions[s.ID] = s
	if _, err := f.svc.Refresh(ctx, "manual-refresh"); !errors.Is(err, domain.ErrSessionInactive) {
		t.Fatalf("refresh for blocked account with active session must fail, got %v", err)
	}
}

func TestModerateBlockRevokesSessionsAndIsIdempotent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s1, _ := f.svc.Register(ctx, "rin@example.com", pw, Client{UserAgent: "phone"})
	s2, _ := f.svc.Login(ctx, "rin@example.com", pw, Client{UserAgent: "laptop"})
	f.pub.events = nil

	opID := uuid.New()
	res, err := f.svc.ModerateBlock(ctx, opID, s1.Account.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Blocked || !res.Changed || res.RevokedSessions != 2 || res.RevocationsPending {
		t.Fatalf("unexpected moderation result: %+v", res)
	}
	if len(f.pub.events) != 2 {
		t.Fatalf("expected 2 SessionRevoked events, got %d", len(f.pub.events))
	}
	for _, ev := range f.pub.events {
		sr, ok := ev.(domain.SessionRevoked)
		if !ok || sr.Session.RevokeReason != domain.RevokeAccountBlocked {
			t.Fatalf("unexpected event: %#v", ev)
		}
	}
	wantTTL := fakeTokens{}.TTL() + time.Minute
	if f.revoked[s1.SessionID.String()] != wantTTL || f.revoked[s2.SessionID.String()] != wantTTL {
		t.Fatalf("expected both sessions revoked in cache with ttl %v, got %+v", wantTTL, f.revoked)
	}

	// Idempotent replay with same operationID does not re-emit events.
	f.pub.events = nil
	replay, err := f.svc.ModerateBlock(ctx, opID, s1.Account.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if replay != res || len(f.pub.events) != 0 {
		t.Fatalf("replay mismatch: %+v vs %+v, events=%d", replay, res, len(f.pub.events))
	}
	if _, err := f.svc.ModerateBlock(ctx, opID, s1.Account.ID, false); !errors.Is(err, domain.ErrOperationConflict) {
		t.Fatalf("expected ErrOperationConflict on mismatched replay, got %v", err)
	}
}

func TestModerateUnblockDoesNotResurrectSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	reg, _ := f.svc.Register(ctx, "rin@example.com", pw, Client{})
	if _, err := f.svc.ModerateBlock(ctx, uuid.New(), reg.Account.ID, true); err != nil {
		t.Fatal(err)
	}
	// Blocking again with a new operationID is a no-op (Changed: false).
	noop, err := f.svc.ModerateBlock(ctx, uuid.New(), reg.Account.ID, true)
	if err != nil || noop.Changed || !noop.Blocked || noop.RevokedSessions != 0 {
		t.Fatalf("expected no-op block, got %+v err=%v", noop, err)
	}

	unblock, err := f.svc.ModerateBlock(ctx, uuid.New(), reg.Account.ID, false)
	if err != nil || !unblock.Changed || unblock.Blocked || unblock.RevokedSessions != 0 {
		t.Fatalf("unexpected unblock result: %+v err=%v", unblock, err)
	}
	if _, err := f.svc.Refresh(ctx, reg.RefreshToken); !errors.Is(err, domain.ErrSessionInactive) {
		t.Fatalf("unblock must not resurrect revoked session, got %v", err)
	}
	if _, err := f.svc.Login(ctx, "rin@example.com", pw, Client{}); err != nil {
		t.Fatalf("unblocked user should be able to login again: %v", err)
	}
}

func TestModerateBlockRejectsAdminAndTracksRedisFailure(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	adminReg, _ := f.svc.Register(ctx, "admin@example.com", pw, Client{})
	adminAcc := f.accounts[adminReg.Account.ID]
	adminAcc.Roles = []string{domain.RoleUser, domain.RoleAdmin}
	f.accounts[adminAcc.ID] = adminAcc

	if _, err := f.svc.ModerateBlock(ctx, uuid.New(), adminAcc.ID, true); !errors.Is(err, domain.ErrCannotBlockAdmin) {
		t.Fatalf("expected ErrCannotBlockAdmin, got %v", err)
	}

	userReg, _ := f.svc.Register(ctx, "user@example.com", pw, Client{})
	f.svc.d.Revocation = failingRevocations{err: errors.New("redis down")}
	res, err := f.svc.ModerateBlock(ctx, uuid.New(), userReg.Account.ID, true)
	if err != nil {
		t.Fatalf("block must succeed even if redis fails: %v", err)
	}
	if !res.RevocationsPending || !res.Changed || res.RevokedSessions != 1 || len(f.pending) != 1 {
		t.Fatalf("expected RevocationsPending=true and 1 pending revocation, got %+v pending=%d", res, len(f.pending))
	}

	// Once Redis recovers, FlushPendingRevocations drains pending revocations.
	f.svc.d.Revocation = f.revoked
	if err := f.svc.FlushPendingRevocations(ctx); err != nil {
		t.Fatalf("FlushPendingRevocations: %v", err)
	}
	if len(f.pending) != 0 || len(f.revoked) != 1 {
		t.Fatalf("expected pending drained and 1 revoked in cache, pending=%d revoked=%d", len(f.pending), len(f.revoked))
	}
}

func TestListAndGetModerationUsers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u1, _ := f.svc.Register(ctx, "a@example.com", pw, Client{})
	u2, _ := f.svc.Register(ctx, "b@example.com", pw, Client{})
	_, _ = f.svc.ModerateBlock(ctx, uuid.New(), u2.Account.ID, true)

	got, err := f.svc.GetModerationUser(ctx, u1.Account.ID)
	if err != nil || got.ID != u1.Account.ID {
		t.Fatalf("GetModerationUser = %+v err=%v", got, err)
	}
	onlyBlocked := true
	list, err := f.svc.ListModerationUsers(ctx, nil, 10, &onlyBlocked)
	if err != nil || len(list) != 1 || list[0].ID != u2.Account.ID {
		t.Fatalf("ListModerationUsers(blocked=true) = %+v err=%v", list, err)
	}
}
