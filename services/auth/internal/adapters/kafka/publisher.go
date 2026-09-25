// Package kafka publishes Auth domain events as versioned contracts.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/authv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

// Producer is the part of the platform producer the publisher needs.
type Producer interface {
	Publish(ctx context.Context, msgs ...platformkafka.Message) error
}

// Publisher implements ports.EventPublisher.
type Publisher struct {
	producer Producer
	source   string
}

// NewPublisher returns a Publisher.
func NewPublisher(p Producer, source string) *Publisher {
	return &Publisher{producer: p, source: source}
}

// Publish maps events to authv1 contracts; the user ID is the partition key.
func (p *Publisher) Publish(ctx context.Context, evs ...domain.Event) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	msgs := make([]platformkafka.Message, 0, len(evs))
	for _, ev := range evs {
		typ, key, at, payload, err := toContract(ev)
		if err != nil {
			return err
		}
		env, err := events.New(typ, authv1.Version, p.source, traceID, at, payload)
		if err != nil {
			return err
		}
		value, err := json.Marshal(env)
		if err != nil {
			return err
		}
		msgs = append(msgs, platformkafka.Message{
			Topic: events.TopicAuthEvents, Key: []byte(key), Value: value,
			Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(authv1.Version), "content-type": "application/json"},
		})
	}
	return p.producer.Publish(ctx, msgs...)
}

func toContract(ev domain.Event) (string, string, time.Time, any, error) {
	switch e := ev.(type) {
	case domain.UserRegistered:
		a := e.Account
		return authv1.TypeUserRegistered, a.ID.String(), a.CreatedAt,
			authv1.UserRegistered{UserID: a.ID.String(), Email: a.Email, RegisteredAt: a.CreatedAt}, nil
	case domain.SessionCreated:
		s := e.Session
		return authv1.TypeSessionCreated, s.UserID.String(), s.CreatedAt,
			authv1.SessionCreated{UserID: s.UserID.String(), SessionID: s.ID.String(), CreatedAt: s.CreatedAt}, nil
	case domain.SessionRevoked:
		s := e.Session
		at := time.Now().UTC()
		if s.RevokedAt != nil {
			at = *s.RevokedAt
		}
		return authv1.TypeSessionRevoked, s.UserID.String(), at,
			authv1.SessionRevoked{UserID: s.UserID.String(), SessionID: s.ID.String(), Reason: s.RevokeReason, RevokedAt: at}, nil
	default:
		return "", "", time.Time{}, nil, fmt.Errorf("unsupported auth event %T", ev)
	}
}

// NopPublisher discards events (Kafka disabled locally).
type NopPublisher struct{}

// Publish implements ports.EventPublisher.
func (NopPublisher) Publish(context.Context, ...domain.Event) error { return nil }
