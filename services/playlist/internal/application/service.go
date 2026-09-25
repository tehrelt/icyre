// Package application implements the playlist use cases.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/playlist/internal/domain"
)

// Repository stores playlists.
type Repository interface {
	Create(ctx context.Context, p domain.Playlist) error
	Get(ctx context.Context, id uuid.UUID) (domain.Playlist, error)
	ByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Playlist, error)
	Tracks(ctx context.Context, id uuid.UUID) ([]domain.Track, error)
	// AppendTrack adds the track at the end; adding it again is a no-op.
	AppendTrack(ctx context.Context, id uuid.UUID, t domain.Track) error
	RemoveTrack(ctx context.Context, id, trackID uuid.UUID, at time.Time) error
}

// Catalog confirms a track exists (domain.ErrTrackNotFound otherwise).
type Catalog interface {
	TrackExists(ctx context.Context, id uuid.UUID) error
}

// Service implements the use cases.
type Service struct {
	repo    Repository
	catalog Catalog
	now     func() time.Time
}

// New returns a Service.
func New(repo Repository, catalog Catalog) *Service {
	return &Service{repo: repo, catalog: catalog, now: time.Now}
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
	return p, s.repo.Create(ctx, p)
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

func (s *Service) owned(ctx context.Context, caller, id uuid.UUID) error {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if p.OwnerID != caller {
		return domain.ErrForbidden
	}
	return nil
}

// AddTrack appends an existing catalog track (idempotent).
func (s *Service) AddTrack(ctx context.Context, caller, id, trackID uuid.UUID) error {
	if err := s.owned(ctx, caller, id); err != nil {
		return err
	}
	if err := s.catalog.TrackExists(ctx, trackID); err != nil {
		return err
	}
	return s.repo.AppendTrack(ctx, id, domain.Track{TrackID: trackID, AddedBy: caller, AddedAt: s.stamp()})
}

// RemoveTrack removes a track (idempotent).
func (s *Service) RemoveTrack(ctx context.Context, caller, id, trackID uuid.UUID) error {
	if err := s.owned(ctx, caller, id); err != nil {
		return err
	}
	return s.repo.RemoveTrack(ctx, id, trackID, s.stamp())
}
