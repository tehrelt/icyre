// Package application implements the library use cases.
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/library/internal/domain"
)

// Repository stores library items.
type Repository interface {
	// Save inserts unless present; Changed reports an insert.
	Save(ctx context.Context, item domain.Item) (domain.Change, error)
	// Remove deletes; Changed reports a delete.
	Remove(ctx context.Context, userID uuid.UUID, kind domain.Kind, id uuid.UUID, at time.Time) (domain.Change, error)
	List(ctx context.Context, userID uuid.UUID, kind domain.Kind, after *domain.Cursor, limit int) ([]domain.Item, error)
	Contains(ctx context.Context, userID uuid.UUID, kind domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error)
	Counts(ctx context.Context, userID uuid.UUID) (domain.Counts, error)
}

// Catalog confirms that a track or album exists (domain.ErrNotFound otherwise).
type Catalog interface {
	Exists(ctx context.Context, kind domain.Kind, id uuid.UUID) error
}

// Publisher announces library changes.
type Publisher interface {
	Saved(ctx context.Context, item domain.Item) error
	Removed(ctx context.Context, item domain.Item) error
}

// Service implements the use cases.
type Service struct {
	repo    Repository
	catalog Catalog
	pub     Publisher
	tx      Transactor
	log     *slog.Logger
	now     func() time.Time
}

// Transactor runs fn as one unit of work: the library change and its event
// (transactional outbox) commit or roll back together.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// New returns a Service.
func New(repo Repository, catalog Catalog, pub Publisher, tx Transactor, log *slog.Logger) *Service {
	if tx == nil {
		tx = noTx{}
	}
	return &Service{repo: repo, catalog: catalog, pub: pub, tx: tx, log: log, now: time.Now}
}

// Save adds a track or album. Only existing catalog items can be saved;
// saving twice keeps the original savedAt and publishes nothing.
func (s *Service) Save(ctx context.Context, userID uuid.UUID, kind domain.Kind, id uuid.UUID) (domain.Item, error) {
	if err := s.catalog.Exists(ctx, kind, id); err != nil {
		return domain.Item{}, err
	}
	var ch domain.Change
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		ch, err = s.repo.Save(ctx, domain.Item{UserID: userID, Kind: kind, EntityID: id, SavedAt: s.now().UTC().Truncate(time.Microsecond)})
		if err != nil || !ch.Changed {
			return err
		}
		return s.pub.Saved(ctx, ch.Item)
	})
	if err != nil {
		return domain.Item{}, err
	}
	return ch.Item, nil
}

// Remove deletes a track or album; removing a missing item is a no-op.
func (s *Service) Remove(ctx context.Context, userID uuid.UUID, kind domain.Kind, id uuid.UUID) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		ch, err := s.repo.Remove(ctx, userID, kind, id, s.now().UTC())
		if err != nil || !ch.Changed {
			return err
		}
		return s.pub.Removed(ctx, ch.Item)
	})
}

// Page is a slice of a newest-first list.
type Page struct {
	Items []domain.Item
	Next  *domain.Cursor
}

// List pages through saved items, newest first.
func (s *Service) List(ctx context.Context, userID uuid.UUID, kind domain.Kind, after *domain.Cursor, limit int) (Page, error) {
	if limit <= 0 {
		limit = domain.DefaultLimit
	}
	limit = min(limit, domain.MaxLimit)
	items, err := s.repo.List(ctx, userID, kind, after, limit+1)
	if err != nil {
		return Page{}, err
	}
	p := Page{Items: items}
	if len(items) > limit {
		p.Items = items[:limit]
		last := p.Items[limit-1]
		p.Next = &domain.Cursor{SavedAt: last.SavedAt, EntityID: last.EntityID}
	}
	return p, nil
}

// Contains returns which of ids are saved (for "liked" marks on pages).
func (s *Service) Contains(ctx context.Context, userID uuid.UUID, kind domain.Kind, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return s.repo.Contains(ctx, userID, kind, ids)
}

// Counts summarises the library (sidebar counters).
func (s *Service) Counts(ctx context.Context, userID uuid.UUID) (domain.Counts, error) {
	return s.repo.Counts(ctx, userID)
}
