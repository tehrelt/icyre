package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	revoked  memRevocations
	pub      *recorder
	now      time.Time
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{accounts: memAccounts{}, sessions: memSessions{}, revoked: memRevocations{}, pub: &recorder{}, now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	svc, err := New(Deps{
		Accounts: f.accounts, Sessions: f.sessions, Hasher: fakeHasher{}, Tokens: fakeTokens{}, Refresh: &seqRefresh{},
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
