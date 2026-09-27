package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/social/internal/domain"
)

type memRepo struct{ edges []domain.Follow }

func (m *memRepo) find(u uuid.UUID, t domain.Target) int {
	return slices.IndexFunc(m.edges, func(f domain.Follow) bool { return f.FollowerID == u && f.Target == t })
}
func (m *memRepo) Follow(_ context.Context, f domain.Follow) (domain.Change, error) {
	if i := m.find(f.FollowerID, f.Target); i >= 0 {
		return domain.Change{Follow: m.edges[i]}, nil
	}
	m.edges = append(m.edges, f)
	return domain.Change{Follow: f, Changed: true}, nil
}
func (m *memRepo) Unfollow(_ context.Context, u uuid.UUID, t domain.Target, at time.Time) (domain.Change, error) {
	f := domain.Follow{FollowerID: u, Target: t, FollowedAt: at}
	i := m.find(u, t)
	if i < 0 {
		return domain.Change{Follow: f}, nil
	}
	m.edges = slices.Delete(m.edges, i, i+1)
	return domain.Change{Follow: f, Changed: true}, nil
}
func (m *memRepo) newest(keep func(domain.Follow) bool, limit int) []domain.Follow {
	var out []domain.Follow
	for i := len(m.edges) - 1; i >= 0; i-- {
		if keep(m.edges[i]) {
			out = append(out, m.edges[i])
		}
	}
	return out[:min(limit, len(out))]
}
func (m *memRepo) Followers(_ context.Context, t domain.Target, _ *domain.Cursor, limit int) ([]domain.Follow, error) {
	return m.newest(func(f domain.Follow) bool { return f.Target == t }, limit), nil
}
func (m *memRepo) Following(_ context.Context, u uuid.UUID, typ domain.TargetType, _ *domain.Cursor, limit int) ([]domain.Follow, error) {
	return m.newest(func(f domain.Follow) bool { return f.FollowerID == u && f.Target.Type == typ }, limit), nil
}
func (m *memRepo) Contains(_ context.Context, u uuid.UUID, typ domain.TargetType, ids []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, id := range ids {
		if m.find(u, domain.Target{Type: typ, ID: id}) >= 0 {
			out = append(out, id)
		}
	}
	return out, nil
}
func (m *memRepo) Counts(_ context.Context, s domain.Target) (domain.Counts, error) {
	var c domain.Counts
	for _, f := range m.edges {
		if f.Target == s {
			c.Followers++
		}
		if s.Type == domain.TargetUser && f.FollowerID == s.ID {
			if f.Target.Type == domain.TargetUser {
				c.FollowingUsers++
			} else {
				c.FollowingArtists++
			}
		}
	}
	return c, nil
}

type fakeDir map[uuid.UUID]bool

func (f fakeDir) Exists(_ context.Context, t domain.Target) error {
	if !f[t.ID] {
		return domain.ErrNotFound
	}
	return nil
}

type recPub struct {
	events []string
	fail   bool
}

func (p *recPub) Followed(_ context.Context, f domain.Follow) error {
	if p.fail {
		return errors.New("outbox down")
	}
	p.events = append(p.events, "followed:"+string(f.Target.Type))
	return nil
}
func (p *recPub) Unfollowed(_ context.Context, f domain.Follow) error {
	p.events = append(p.events, "unfollowed:"+string(f.Target.Type))
	return nil
}

func newService(dir fakeDir) (*Service, *recPub) {
	pub := &recPub{}
	s := New(&memRepo{}, dir, pub, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	tick := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { tick = tick.Add(time.Second); return tick }
	return s, pub
}

func TestFollowUnfollow(t *testing.T) {
	me, friend, artist := uuid.New(), uuid.New(), uuid.New()
	s, pub := newService(fakeDir{friend: true, artist: true})
	ctx := context.Background()
	user := domain.Target{Type: domain.TargetUser, ID: friend}
	art := domain.Target{Type: domain.TargetArtist, ID: artist}

	first, err := s.Follow(ctx, me, user)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Follow(ctx, me, user)
	if err != nil || !again.FollowedAt.Equal(first.FollowedAt) {
		t.Fatalf("repeat follow: %+v %v", again, err)
	}
	if _, err := s.Follow(ctx, me, art); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Follow(ctx, me, domain.Target{Type: domain.TargetUser, ID: me}); !errors.Is(err, domain.ErrSelfFollow) {
		t.Fatalf("self follow: %v", err)
	}
	if _, err := s.Follow(ctx, me, domain.Target{Type: domain.TargetArtist, ID: uuid.New()}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown artist: %v", err)
	}
	c, _ := s.Counts(ctx, domain.Target{Type: domain.TargetUser, ID: me})
	if c.FollowingUsers != 1 || c.FollowingArtists != 1 {
		t.Fatalf("counts: %+v", c)
	}
	if got, _ := s.Contains(ctx, me, domain.TargetArtist, []uuid.UUID{artist, uuid.New()}); !slices.Equal(got, []uuid.UUID{artist}) {
		t.Fatalf("contains: %v", got)
	}
	for range 2 {
		if err := s.Unfollow(ctx, me, user); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"followed:user", "followed:artist", "unfollowed:user"}
	if !slices.Equal(pub.events, want) {
		t.Fatalf("events: %v", pub.events)
	}
}

func TestFollowPublishFailure(t *testing.T) {
	friend := uuid.New()
	s, pub := newService(fakeDir{friend: true})
	pub.fail = true
	if _, err := s.Follow(context.Background(), uuid.New(), domain.Target{Type: domain.TargetUser, ID: friend}); err == nil {
		t.Fatal("publish failure must fail the follow")
	}
}

func TestPages(t *testing.T) {
	me := uuid.New()
	dir := fakeDir{}
	s, _ := newService(dir)
	ctx := context.Background()
	for range 3 {
		id := uuid.New()
		dir[id] = true
		if _, err := s.Follow(ctx, me, domain.Target{Type: domain.TargetArtist, ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.Following(ctx, me, domain.TargetArtist, nil, 2)
	if err != nil || len(p.Follows) != 2 || p.Next == nil || p.Next.ID != p.Follows[1].Target.ID {
		t.Fatalf("following page: %+v %v", p, err)
	}
	target := p.Follows[0].Target
	p, err = s.Followers(ctx, target, nil, 0)
	if err != nil || len(p.Follows) != 1 || p.Next != nil || p.Follows[0].FollowerID != me {
		t.Fatalf("followers page: %+v %v", p, err)
	}
}
