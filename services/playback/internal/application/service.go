// Package application accepts playback telemetry and publishes it.
package application

import (
	"context"
	"time"

	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

// Publisher sends playback events to the event bus.
type Publisher interface {
	Publish(ctx context.Context, e domain.Event) error
}

// Service implements the use case.
type Service struct {
	pub Publisher
	now func() time.Time
}

// New returns a Service.
func New(pub Publisher) *Service { return &Service{pub: pub, now: time.Now} }

// Report validates a player report, stamps it with server time and
// publishes it. The server clock is authoritative: client clocks drift.
func (s *Service) Report(ctx context.Context, e domain.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	e.At = s.now().UTC()
	return s.pub.Publish(ctx, e)
}
