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
	// Tx is the unit of work; nil runs without a transaction (tests).
	Tx  ports.Transactor
	Log *slog.Logger
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
	if d.Tx == nil {
		d.Tx = noTx{}
	}
	return &Service{d: d}
}

// store runs write and records events in one transaction (transactional
// outbox): an event exists if and only if its change committed, and the
// outbox relay delivers it to Kafka at least once.
func (s *Service) store(ctx context.Context, write func(ctx context.Context) error, events ...domain.Event) error {
	return s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := write(ctx); err != nil {
			return err
		}
		if err := s.d.Publisher.Publish(ctx, events...); err != nil {
			return fmt.Errorf("record events: %w", err)
		}
		return nil
	})
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

// noTx runs the unit of work directly (in-memory test repositories).
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
