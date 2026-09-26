// Package kafka decodes playback.events records for the batch consumer.
package kafka

import (
	"context"
	"errors"

	"github.com/tehrelt/icyre/libs/contracts/events"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/analytics/internal/domain"
)

// Decode turns a record into an Event. Event types analytics does not count
// are skipped; any other error dead-letters the record (decoding depends on
// nothing but the bytes).
func Decode(_ context.Context, rec platformkafka.Record) (domain.Event, error) {
	env, err := events.Decode(rec.Value)
	if err != nil {
		return domain.Event{}, err
	}
	e, err := domain.FromEnvelope(env)
	if errors.Is(err, domain.ErrUnknownType) {
		return domain.Event{}, platformkafka.ErrSkip
	}
	return e, err
}
