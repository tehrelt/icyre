// Package application implements Catalog use cases on top of the domain
// model and the ports. It knows nothing about transport or storage.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// Deps are the collaborators of Service.
type Deps struct {
	Artists   ports.ArtistRepository
	Albums    ports.AlbumRepository
	Tracks    ports.TrackRepository
	Genres    ports.GenreRepository
	Publisher ports.EventPublisher
	Log       *slog.Logger
	// Now and NewID are overridable for tests.
	Now   func() time.Time
	NewID func() (uuid.UUID, error)
}

// Service exposes the Catalog use cases.
type Service struct {
	d Deps
}

// New returns a Service. Now defaults to time.Now, NewID to UUIDv7.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.NewID == nil {
		d.NewID = uuid.NewV7
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{d: d}
}

// publish delivers events after the state change is committed.
//
// Publishing is best effort: the write already succeeded, so a broker
// failure is logged instead of failing the request. A transactional outbox
// will make this at-least-once end to end (see services/catalog/README.md).
func (s *Service) publish(ctx context.Context, events ...domain.Event) {
	if err := s.d.Publisher.Publish(ctx, events...); err != nil {
		s.d.Log.ErrorContext(ctx, "publish catalog events failed", "error", err, "count", len(events))
	}
}

func (s *Service) newID() (uuid.UUID, error) {
	id, err := s.d.NewID()
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate id: %w", err)
	}
	return id, nil
}

// asReference turns a not-found error of a referenced entity into a
// ReferenceError pointing at the request field.
func asReference(err error, field string, target error) error {
	if errors.Is(err, target) {
		return &domain.ReferenceError{Field: field, Err: target}
	}
	return err
}
