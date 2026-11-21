// Package application manages playback sessions, queues, and telemetry.
package application

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

// Publisher sends playback events to the event bus.
type Publisher interface {
	Publish(ctx context.Context, e domain.Event) error
}

// StateStore persists transient playback sessions, queues, and progress throttles.
type StateStore interface {
	GetState(ctx context.Context, userID uuid.UUID) (domain.Session, domain.Queue, error)
	SaveState(ctx context.Context, sess domain.Session, q domain.Queue) error
	AllowProgress(ctx context.Context, userID, playbackID uuid.UUID, interval time.Duration) (bool, error)
}

const defaultProgressInterval = 10 * time.Second

// Service implements the playback use cases.
type Service struct {
	pub              Publisher
	store            StateStore
	progressInterval time.Duration
	log              *slog.Logger
	now              func() time.Time
	shuffle          domain.ShuffleFunc
}

// New returns a Service.
func New(pub Publisher) *Service {
	return &Service{
		pub:              pub,
		progressInterval: defaultProgressInterval,
		now:              time.Now,
		shuffle:          rand.Shuffle,
	}
}

// WithStateStore configures Redis-backed session, queue, and progress throttling.
func (s *Service) WithStateStore(store StateStore, progressInterval time.Duration, log *slog.Logger) *Service {
	s.store = store
	if progressInterval > 0 {
		s.progressInterval = progressInterval
	}
	s.log = log
	return s
}

// Report validates a player report, stamps it with server time, throttles
// progress events, and publishes it. The server clock is authoritative.
func (s *Service) Report(ctx context.Context, e domain.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	e.At = s.now().UTC()
	if e.Kind == domain.Progress && !s.allowProgress(ctx, e.UserID, e.PlaybackID) {
		return nil
	}
	return s.pub.Publish(ctx, e)
}

// StartSession creates a new playback session and queue for the user.
func (s *Service) StartSession(ctx context.Context, userID uuid.UUID, req domain.StartSessionRequest) (domain.PlaybackState, error) {
	if s.store == nil {
		return domain.PlaybackState{}, domain.ErrStoreUnavailable
	}
	sess, q, tr, err := domain.NewSession(userID, req, s.now(), s.shuffle)
	if err != nil {
		return domain.PlaybackState{}, err
	}
	if err := s.store.SaveState(ctx, sess, q); err != nil {
		return domain.PlaybackState{}, err
	}
	s.publishBestEffort(ctx, tr.Events)
	return domain.PlaybackState{Session: sess, Queue: q}, nil
}

// GetSession returns the user's active playback session and queue.
func (s *Service) GetSession(ctx context.Context, userID uuid.UUID) (domain.PlaybackState, error) {
	if s.store == nil {
		return domain.PlaybackState{}, domain.ErrStoreUnavailable
	}
	sess, q, err := s.store.GetState(ctx, userID)
	if err != nil {
		return domain.PlaybackState{}, err
	}
	return domain.PlaybackState{Session: sess, Queue: q}, nil
}

// SetQueue replaces the user's playback queue.
func (s *Service) SetQueue(ctx context.Context, userID uuid.UUID, req domain.SetQueueRequest) (domain.PlaybackState, error) {
	if s.store == nil {
		return domain.PlaybackState{}, domain.ErrStoreUnavailable
	}
	sess, q, err := s.store.GetState(ctx, userID)
	if err != nil {
		return domain.PlaybackState{}, err
	}
	nextSess, nextQ, tr, err := domain.ApplyQueue(sess, q, req, s.now(), s.shuffle)
	if err != nil {
		return domain.PlaybackState{}, err
	}
	if err := s.store.SaveState(ctx, nextSess, nextQ); err != nil {
		return domain.PlaybackState{}, err
	}
	s.publishBestEffort(ctx, tr.Events)
	return domain.PlaybackState{Session: nextSess, Queue: nextQ}, nil
}

// UpdateState applies a partial playback state update (play/pause, seek, shuffle, repeat, next/previous).
func (s *Service) UpdateState(ctx context.Context, userID uuid.UUID, patch domain.StatePatch) (domain.PlaybackState, error) {
	if s.store == nil {
		return domain.PlaybackState{}, domain.ErrStoreUnavailable
	}
	sess, q, err := s.store.GetState(ctx, userID)
	if err != nil {
		return domain.PlaybackState{}, err
	}
	nextSess, nextQ, tr, err := domain.ApplyState(sess, q, patch, s.now(), s.shuffle)
	if err != nil {
		return domain.PlaybackState{}, err
	}
	if err := s.store.SaveState(ctx, nextSess, nextQ); err != nil {
		return domain.PlaybackState{}, err
	}
	s.publishBestEffort(ctx, tr.Events)
	return domain.PlaybackState{Session: nextSess, Queue: nextQ}, nil
}

func (s *Service) allowProgress(ctx context.Context, userID, playbackID uuid.UUID) bool {
	if s.store == nil {
		return true
	}
	allowed, err := s.store.AllowProgress(ctx, userID, playbackID, s.progressInterval)
	if err != nil {
		if s.log != nil {
			s.log.WarnContext(ctx, "progress throttle check failed", "error", err)
		}
		return true
	}
	return allowed
}

func (s *Service) publishBestEffort(ctx context.Context, events []domain.Event) {
	for _, e := range events {
		if e.Kind == domain.Progress && !s.allowProgress(ctx, e.UserID, e.PlaybackID) {
			continue
		}
		if err := s.pub.Publish(ctx, e); err != nil && s.log != nil {
			s.log.WarnContext(ctx, "publish playback transition event failed", "type", e.Kind, "error", err)
		}
	}
}
