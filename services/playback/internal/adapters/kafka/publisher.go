// Package kafka publishes playback.events.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
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
func NewPublisher(p Producer, source string) *Publisher { return &Publisher{producer: p, source: source} }

var types = map[domain.Kind]string{
	domain.Started:  playbackv1.TypeStarted,
	domain.Finished: playbackv1.TypeFinished,
	domain.Skipped:  playbackv1.TypeSkipped,
}

// Publish sends one event keyed by user ID.
func (p *Publisher) Publish(ctx context.Context, e domain.Event) error {
	typ := types[e.Kind]
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	env, err := events.New(typ, playbackv1.Version, p.source, traceID, e.At, playbackv1.Playback{
		PlaybackID: e.PlaybackID.String(), UserID: e.UserID.String(), TrackID: e.TrackID.String(),
		Source: e.Source, DurationMs: e.DurationMs, ListenedMs: e.ListenedMs, At: e.At,
	})
	if err != nil {
		return err
	}
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicPlaybackEvents, Key: []byte(e.UserID.String()), Value: value,
		Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(playbackv1.Version), "content-type": "application/json"},
	})
}
