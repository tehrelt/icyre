package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

type memRepo map[uuid.UUID]domain.Profile

func (m memRepo) taken(p domain.Profile) bool {
	for id, x := range m {
		if id != p.UserID && x.Username == p.Username {
			return true
		}
	}
	return false
}

func (m memRepo) CreateIfAbsent(_ context.Context, p domain.Profile) (bool, error) {
	if _, ok := m[p.UserID]; ok {
		return false, nil
	}
	if m.taken(p) {
		return false, domain.ErrUsernameTaken
	}
	m[p.UserID] = p
	return true, nil
}
func (m memRepo) Get(_ context.Context, id uuid.UUID) (domain.Profile, error) {
	p, ok := m[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return p, nil
}
func (m memRepo) Update(_ context.Context, p domain.Profile) error {
	if m.taken(p) {
		return domain.ErrUsernameTaken
	}
	m[p.UserID] = p
	return nil
}

type pubRecorder struct{ n int }

func (p *pubRecorder) ProfileUpdated(context.Context, domain.Profile) error { p.n++; return nil }

func newSvc() (*Service, memRepo, *pubRecorder) {
	repo, pub := memRepo{}, &pubRecorder{}
	return New(repo, pub, nil, slog.New(slog.NewTextHandler(io.Discard, nil))), repo, pub
}

func TestCreateForNewUserIsIdempotent(t *testing.T) {
	svc, repo, pub := newSvc()
	id := uuid.New()
	for i := 0; i < 3; i++ { // redelivered event
		if err := svc.CreateForNewUser(context.Background(), id, "rin@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo) != 1 || pub.n != 1 {
		t.Fatalf("profiles=%d events=%d", len(repo), pub.n)
	}
}

func TestUsernameCollisionsGetSuffixes(t *testing.T) {
	svc, repo, _ := newSvc()
	ctx := context.Background()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	_ = svc.CreateForNewUser(ctx, a, "rin@one.com")
	_ = svc.CreateForNewUser(ctx, b, "rin@two.com")
	_ = svc.CreateForNewUser(ctx, c, "rin@three.com")
	if repo[a].Username != "rin" || repo[b].Username != "rin2" || repo[c].Username != "rin3" {
		t.Fatalf("usernames %s %s %s", repo[a].Username, repo[b].Username, repo[c].Username)
	}
}

// countingTx counts units of work and the ones that failed (rolled back).
type countingTx struct{ runs, rolledBack int }

func (c *countingTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	c.runs++
	err := fn(ctx)
	if err != nil {
		c.rolledBack++
	}
	return err
}

type failingPub struct{}

func (failingPub) ProfileUpdated(context.Context, domain.Profile) error {
	return errors.New("outbox insert failed")
}

func TestEachUsernameAttemptIsItsOwnTransaction(t *testing.T) {
	repo, tx := memRepo{}, &countingTx{}
	svc := New(repo, &pubRecorder{}, tx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	_ = svc.CreateForNewUser(ctx, uuid.New(), "rin@example.com")
	// The clash rolls its transaction back; the retry runs in a fresh one.
	if err := svc.CreateForNewUser(ctx, uuid.New(), "rin@example.org"); err != nil || tx.runs != 3 || tx.rolledBack != 1 {
		t.Fatalf("err %v, tx %+v", err, tx)
	}

	tx = &countingTx{}
	svc = New(memRepo{}, failingPub{}, tx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := svc.CreateForNewUser(ctx, uuid.New(), "kai@example.com"); err == nil || tx.rolledBack != 1 {
		t.Fatalf("a failed event write must fail (and retry via Kafka): %v, %+v", err, tx)
	}
}

func TestUpdate(t *testing.T) {
	svc, _, pub := newSvc()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	_ = svc.CreateForNewUser(ctx, a, "rin@example.com")
	_ = svc.CreateForNewUser(ctx, b, "kai@example.com")

	name := "Rin Aoki"
	p, err := svc.Update(ctx, a, domain.Changes{DisplayName: &name})
	if err != nil || p.DisplayName != "Rin Aoki" || pub.n != 3 {
		t.Fatalf("p=%+v err=%v events=%d", p, err, pub.n)
	}
	taken := "kai"
	if _, err := svc.Update(ctx, a, domain.Changes{Username: &taken}); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
	if _, err := svc.Update(ctx, uuid.New(), domain.Changes{DisplayName: &name}); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing profile: %v", err)
	}
}
