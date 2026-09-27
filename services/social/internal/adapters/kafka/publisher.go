// Package kafka publishes social.events.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/socialv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/social/internal/domain"
)

// Producer is the part of the platform producer used here.
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

func (p *Publisher) publish(ctx context.Context, f domain.Follow, typ string, payload any) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	env, err := events.New(typ, socialv1.Version, p.source, traceID, f.FollowedAt, payload)
	if err != nil {
		return err
	}
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}
	// Keyed by follower: one listener's follows and unfollows stay in order.
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicSocialEvents, Key: []byte(f.FollowerID.String()), Value: value,
		Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(socialv1.Version), "content-type": "application/json"},
	})
}

// Followed publishes social.followed.
func (p *Publisher) Followed(ctx context.Context, f domain.Follow) error {
	return p.publish(ctx, f, socialv1.TypeFollowed, socialv1.Followed{
		FollowerID: f.FollowerID.String(), TargetType: string(f.Target.Type), TargetID: f.Target.ID.String(), FollowedAt: f.FollowedAt,
	})
}

// Unfollowed publishes social.unfollowed; Follow.FollowedAt carries the removal time.
func (p *Publisher) Unfollowed(ctx context.Context, f domain.Follow) error {
	return p.publish(ctx, f, socialv1.TypeUnfollowed, socialv1.Unfollowed{
		FollowerID: f.FollowerID.String(), TargetType: string(f.Target.Type), TargetID: f.Target.ID.String(), UnfollowedAt: f.FollowedAt,
	})
}

// NopPublisher discards events.
type NopPublisher struct{}

// Followed implements application.Publisher.
func (NopPublisher) Followed(context.Context, domain.Follow) error { return nil }

// Unfollowed implements application.Publisher.
func (NopPublisher) Unfollowed(context.Context, domain.Follow) error { return nil }
