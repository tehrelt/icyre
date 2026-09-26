// Package application implements the playlist use cases.
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

// Repository stores playlists.
type Repository interface {
	Create(ctx context.Context, p domain.Playlist) error
	Get(ctx context.Context, id uuid.UUID) (domain.Playlist, error)
	ByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Playlist, error)
	// Page lists playlists by ID, after the given one.
	Page(ctx context.Context, after uuid.UUID, limit int) ([]domain.Playlist, error)
	Tracks(ctx context.Context, id uuid.UUID) ([]domain.Track, error)
	UpdateTitle(ctx context.Context, id uuid.UUID, title string, at time.Time) error
	// Delete removes the playlist with its tracks; false if it was already gone.
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	// AppendTrack adds the track at the end and returns it with its position;
	// adding it again is a no-op (false).
	AppendTrack(ctx context.Context, id uuid.UUID, t domain.Track) (domain.Track, bool, error)
	// RemoveTrack deletes the track; false if it was not in the playlist.
	RemoveTrack(ctx context.Context, id, trackID uuid.UUID, at time.Time) (bool, error)
	// Reorder sets positions 1..n in the given order. The order must list
	// exactly the current tracks (domain.ErrOrderMismatch otherwise); the check
	// and the update happen under one lock.
	Reorder(ctx context.Context, id uuid.UUID, order []uuid.UUID, at time.Time) error
}

// Catalog confirms a track exists (domain.ErrTrackNotFound otherwise).
type Catalog interface {
	TrackExists(ctx context.Context, id uuid.UUID) error
}

// Publisher announces playlist changes (playlist.events).
type Publisher interface {
	Created(ctx context.Context, p domain.Playlist) error
	Updated(ctx context.Context, p domain.Playlist) error
	Deleted(ctx context.Context, p domain.Playlist, at time.Time) error
	TrackAdded(ctx context.Context, p domain.Playlist, t domain.Track) error
	TrackRemoved(ctx context.Context, p domain.Playlist, trackID uuid.UUID, at time.Time) error
	TracksReordered(ctx context.Context, p domain.Playlist, order []uuid.UUID, at time.Time) error
}

// Transactor runs fn as one unit of work: a playlist change and its event
// (transactional outbox) commit or roll back together.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Service implements the use cases.
type Service struct {
	repo    Repository
	catalog Catalog
	pub     Publisher
	tx      Transactor
	log     *slog.Logger
	now     func() time.Time
}

// New returns a Service; a nil tx runs without a transaction (tests).
func New(repo Repository, catalog Catalog, pub Publisher, tx Transactor, log *slog.Logger) *Service {
	if tx == nil {
		tx = noTx{}
	}
	return &Service{repo: repo, catalog: catalog, pub: pub, tx: tx, log: log, now: time.Now}
}

func (s *Service) stamp() time.Time { return s.now().UTC().Truncate(time.Microsecond) }

// Create makes an empty playlist owned by the caller.
func (s *Service) Create(ctx context.Context, owner uuid.UUID, title string) (domain.Playlist, error) {
	t, err := domain.NormalizeTitle(title)
	if err != nil {
		return domain.Playlist{}, err
	}
	now := s.stamp()
	p := domain.Playlist{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Title: t, CreatedAt: now, UpdatedAt: now}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, p); err != nil {
			return err
		}
		return s.pub.Created(ctx, p)
	})
	if err != nil {
		return domain.Playlist{}, err
	}
	return p, nil
}

// Get returns a playlist with its tracks in order.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Playlist, []domain.Track, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return p, nil, err
	}
	tracks, err := s.repo.Tracks(ctx, id)
	return p, tracks, err
}

// Mine lists the caller's playlists, recently changed first.
func (s *Service) Mine(ctx context.Context, owner uuid.UUID) ([]domain.Playlist, error) {
	return s.repo.ByOwner(ctx, owner)
}

// MaxPage bounds All.
const MaxPage = 500

// All pages through every playlist by ID (index rebuilds); after = uuid.Nil
// starts from the beginning.
func (s *Service) All(ctx context.Context, after uuid.UUID, limit int) ([]domain.Playlist, error) {
	if limit <= 0 || limit > MaxPage {
		limit = MaxPage
	}
	return s.repo.Page(ctx, after, limit)
}

func (s *Service) owned(ctx context.Context, caller, id uuid.UUID) (domain.Playlist, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return p, err
	}
	if p.OwnerID != caller {
		return domain.Playlist{}, domain.ErrForbidden
	}
	return p, nil
}

// Rename changes the title; the same title is a no-op.
func (s *Service) Rename(ctx context.Context, caller, id uuid.UUID, title string) (domain.Playlist, error) {
	t, err := domain.NormalizeTitle(title)
	if err != nil {
		return domain.Playlist{}, err
	}
	p, err := s.owned(ctx, caller, id)
	if err != nil || p.Title == t {
		return p, err
	}
	p.Title, p.UpdatedAt = t, s.stamp()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateTitle(ctx, id, p.Title, p.UpdatedAt); err != nil {
			return err
		}
		return s.pub.Updated(ctx, p)
	})
	if err != nil {
		return domain.Playlist{}, err
	}
	return p, nil
}

// Delete removes the caller's playlist.
func (s *Service) Delete(ctx context.Context, caller, id uuid.UUID) error {
	p, err := s.owned(ctx, caller, id)
	if err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		deleted, err := s.repo.Delete(ctx, id)
		if err != nil || !deleted {
			return err
		}
		return s.pub.Deleted(ctx, p, s.stamp())
	})
}

// AddTrack appends an existing catalog track (idempotent).
func (s *Service) AddTrack(ctx context.Context, caller, id, trackID uuid.UUID) error {
	p, err := s.owned(ctx, caller, id)
	if err != nil {
		return err
	}
	// The Catalog call stays outside the transaction: no lock is held
	// across a network round trip.
	if err := s.catalog.TrackExists(ctx, trackID); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		t, added, err := s.repo.AppendTrack(ctx, id, domain.Track{TrackID: trackID, AddedBy: caller, AddedAt: s.stamp()})
		if err != nil || !added {
			return err
		}
		return s.pub.TrackAdded(ctx, p, t)
	})
}

// RemoveTrack removes a track (idempotent).
func (s *Service) RemoveTrack(ctx context.Context, caller, id, trackID uuid.UUID) error {
	p, err := s.owned(ctx, caller, id)
	if err != nil {
		return err
	}
	now := s.stamp()
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		removed, err := s.repo.RemoveTrack(ctx, id, trackID, now)
		if err != nil || !removed {
			return err
		}
		return s.pub.TrackRemoved(ctx, p, trackID, now)
	})
}

// Reorder puts the tracks in the given order (every track exactly once).
func (s *Service) Reorder(ctx context.Context, caller, id uuid.UUID, order []uuid.UUID) error {
	p, err := s.owned(ctx, caller, id)
	if err != nil {
		return err
	}
	now := s.stamp()
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Reorder(ctx, id, order, now); err != nil {
			return err
		}
		return s.pub.TracksReordered(ctx, p, order, now)
	})
}
