// Package application implements the social graph use cases.
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/social/internal/domain"
)

// Repository stores follow edges and their counters.
type Repository interface {
	// Follow inserts unless present and bumps the counters; Changed reports an insert.
	Follow(ctx context.Context, f domain.Follow) (domain.Change, error)
	// Unfollow deletes and decrements the counters; Changed reports a delete.
	Unfollow(ctx context.Context, follower uuid.UUID, target domain.Target, at time.Time) (domain.Change, error)
	Followers(ctx context.Context, target domain.Target, after *domain.Cursor, limit int) ([]domain.Follow, error)
	Following(ctx context.Context, follower uuid.UUID, typ domain.TargetType, after *domain.Cursor, limit int) ([]domain.Follow, error)
	Contains(ctx context.Context, follower uuid.UUID, typ domain.TargetType, ids []uuid.UUID) ([]uuid.UUID, error)
	Counts(ctx context.Context, subject domain.Target) (domain.Counts, error)
}

// Directory confirms that a user (User Profile) or an artist (Catalog)
// exists; domain.ErrNotFound otherwise.
type Directory interface {
	Exists(ctx context.Context, target domain.Target) error
}

// Publisher announces graph changes.
type Publisher interface {
	Followed(ctx context.Context, f domain.Follow) error
	Unfollowed(ctx context.Context, f domain.Follow) error
}

// Transactor runs fn as one unit of work: the edge change and its event
// (transactional outbox) commit or roll back together.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Service implements the use cases.
type Service struct {
	repo Repository
	dir  Directory
	pub  Publisher
	tx   Transactor
	log  *slog.Logger
	now  func() time.Time
}

// New returns a Service.
func New(repo Repository, dir Directory, pub Publisher, tx Transactor, log *slog.Logger) *Service {
	if tx == nil {
		tx = noTx{}
	}
	return &Service{repo: repo, dir: dir, pub: pub, tx: tx, log: log, now: time.Now}
}

// Follow makes follower follow target. Only existing users and artists can be
// followed, never yourself; following twice keeps the original followedAt
// and publishes nothing.
func (s *Service) Follow(ctx context.Context, follower uuid.UUID, target domain.Target) (domain.Follow, error) {
	if target.Type == domain.TargetUser && target.ID == follower {
		return domain.Follow{}, domain.ErrSelfFollow
	}
	if err := s.dir.Exists(ctx, target); err != nil {
		return domain.Follow{}, err
	}
	var ch domain.Change
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		ch, err = s.repo.Follow(ctx, domain.Follow{FollowerID: follower, Target: target, FollowedAt: s.now().UTC().Truncate(time.Microsecond)})
		if err != nil || !ch.Changed {
			return err
		}
		return s.pub.Followed(ctx, ch.Follow)
	})
	if err != nil {
		return domain.Follow{}, err
	}
	return ch.Follow, nil
}

// Unfollow removes the edge; unfollowing a target that is not followed is a no-op.
func (s *Service) Unfollow(ctx context.Context, follower uuid.UUID, target domain.Target) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.repo.Unfollow(ctx, follower, target, s.now().UTC().Truncate(time.Microsecond))
		if err != nil || !ch.Changed {
			return err
		}
		return s.pub.Unfollowed(ctx, ch.Follow)
	})
}

// Page is a slice of a newest-first list.
type Page struct {
	Follows []domain.Follow
	Next    *domain.Cursor
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return domain.DefaultLimit
	}
	return min(limit, domain.MaxLimit)
}

// page trims one look-ahead row into a cursor; key picks the edge end the
// list is ordered by.
func page(rows []domain.Follow, limit int, key func(domain.Follow) uuid.UUID) Page {
	p := Page{Follows: rows}
	if len(rows) > limit {
		p.Follows = rows[:limit]
		last := p.Follows[limit-1]
		p.Next = &domain.Cursor{At: last.FollowedAt, ID: key(last)}
	}
	return p
}

// Followers pages through who follows target, newest first.
func (s *Service) Followers(ctx context.Context, target domain.Target, after *domain.Cursor, limit int) (Page, error) {
	limit = clampLimit(limit)
	rows, err := s.repo.Followers(ctx, target, after, limit+1)
	if err != nil {
		return Page{}, err
	}
	return page(rows, limit, func(f domain.Follow) uuid.UUID { return f.FollowerID }), nil
}

// Following pages through the users or artists follower follows, newest first.
func (s *Service) Following(ctx context.Context, follower uuid.UUID, typ domain.TargetType, after *domain.Cursor, limit int) (Page, error) {
	limit = clampLimit(limit)
	rows, err := s.repo.Following(ctx, follower, typ, after, limit+1)
	if err != nil {
		return Page{}, err
	}
	return page(rows, limit, func(f domain.Follow) uuid.UUID { return f.Target.ID }), nil
}

// Contains returns which of ids follower follows (for "Following" buttons).
func (s *Service) Contains(ctx context.Context, follower uuid.UUID, typ domain.TargetType, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return s.repo.Contains(ctx, follower, typ, ids)
}

// Counts returns the follow counters of a user or an artist; unknown
// subjects have zero counters.
func (s *Service) Counts(ctx context.Context, subject domain.Target) (domain.Counts, error) {
	return s.repo.Counts(ctx, subject)
}
