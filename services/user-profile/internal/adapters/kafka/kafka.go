// Package kafka connects User Profile to the event bus: it consumes
// auth.events (user.registered) and publishes profile.events.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/authv1"
	"github.com/tehrelt/icyre/libs/contracts/events/profilev1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

// Registrations is the use case the consumer drives.
type Registrations interface {
	CreateForNewUser(ctx context.Context, userID uuid.UUID, email string) error
}

// AuthEventsHandler returns the handler for auth.events. Only user.registered
// matters here; other types are acknowledged and skipped. Undecodable
// messages are permanent failures (straight to the DLQ, no retries).
func AuthEventsHandler(app Registrations) platformkafka.Handler {
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		if env.EventType != authv1.TypeUserRegistered {
			return nil
		}
		if env.EventVersion != authv1.Version {
			return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
		}
		var p authv1.UserRegistered
		if err := env.DecodePayload(&p); err != nil {
			return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}
		id, err := uuid.Parse(p.UserID)
		if err != nil {
			return fmt.Errorf("%w: userId: %v", platformkafka.ErrPermanent, err)
		}
		// Idempotent: a redelivered event finds the profile already there.
		return app.CreateForNewUser(ctx, id, p.Email)
	}
}

// Producer is the part of the platform producer the publisher needs.
type Producer interface {
	Publish(ctx context.Context, msgs ...platformkafka.Message) error
}

// Publisher implements application.Publisher.
type Publisher struct {
	producer Producer
	source   string
}

// NewPublisher returns a Publisher.
func NewPublisher(p Producer, source string) *Publisher {
	return &Publisher{producer: p, source: source}
}

// ProfileUpdated publishes profile.updated keyed by user ID.
func (p *Publisher) ProfileUpdated(ctx context.Context, pr domain.Profile) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	env, err := events.New(profilev1.TypeProfileUpdated, profilev1.Version, p.source, traceID, pr.UpdatedAt, profilev1.ProfileUpdated{
		UserID: pr.UserID.String(), Username: pr.Username, DisplayName: pr.DisplayName,
		AvatarKey: pr.AvatarKey, Country: pr.Country, Language: pr.Language, UpdatedAt: pr.UpdatedAt,
	})
	if err != nil {
		return err
	}
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicProfileEvents, Key: []byte(pr.UserID.String()), Value: value,
		Headers: map[string]string{"event-type": profilev1.TypeProfileUpdated, "event-version": fmt.Sprint(profilev1.Version), "content-type": "application/json"},
	})
}

// NopPublisher discards events.
type NopPublisher struct{}

// ProfileUpdated implements application.Publisher.
func (NopPublisher) ProfileUpdated(context.Context, domain.Profile) error { return nil }
