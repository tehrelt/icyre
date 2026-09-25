// Package application records listens and reads the history.
package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/history/internal/domain"
)

// Repository stores listens.
type Repository interface {
	// Record inserts unless the playback is already recorded (idempotent).
	Record(ctx context.Context, l domain.Listen) (bool, error)
	Tracks(ctx context.Context, userID uuid.UUID, after *domain.Cursor, limit int) ([]domain.Listen, error)
	RecentSources(ctx context.Context, userID uuid.UUID, limit int) ([]domain.RecentSource, error)
}

// Service implements the use cases.
type Service struct {
	repo Repository
	rule domain.Rule
}

// New returns a Service applying rule.
func New(repo Repository, rule domain.Rule) *Service { return &Service{repo: repo, rule: rule} }

// PlayEnded handles a finished or skipped play: it becomes a listen when it
// satisfies the rule. A redelivered event is a no-op.
func (s *Service) PlayEnded(ctx context.Context, l domain.Listen) (recorded bool, err error) {
	if !s.rule.Counts(l.ListenedMs, l.DurationMs) {
		return false, nil
	}
	return s.repo.Record(ctx, l)
}

// Limits.
const (
	DefaultLimit = 50
	MaxLimit     = 100
)

// Page is a slice of the newest-first history.
type Page struct {
	Listens []domain.Listen
	Next    *domain.Cursor
}

// Tracks pages through listens, newest first.
func (s *Service) Tracks(ctx context.Context, userID uuid.UUID, after *domain.Cursor, limit int) (Page, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	rows, err := s.repo.Tracks(ctx, userID, after, limit+1)
	if err != nil {
		return Page{}, err
	}
	p := Page{Listens: rows}
	if len(rows) > limit {
		p.Listens = rows[:limit]
		last := p.Listens[limit-1]
		p.Next = &domain.Cursor{PlayedAt: last.PlayedAt, PlaybackID: last.PlaybackID}
	}
	return p, nil
}

// RecentSources returns the collections played from most recently
// ("Recently played" on Home), without duplicates.
func (s *Service) RecentSources(ctx context.Context, userID uuid.UUID, limit int) ([]domain.RecentSource, error) {
	if limit <= 0 {
		limit = 8
	}
	return s.repo.RecentSources(ctx, userID, min(limit, 50))
}
