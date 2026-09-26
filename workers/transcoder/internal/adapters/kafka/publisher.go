package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
)

// Producer is the part of the platform producer used here.
type Producer interface {
	Publish(ctx context.Context, msgs ...platformkafka.Message) error
}

// Publisher implements application.Publisher. It publishes directly (the
// worker has no database for an outbox): the offset of track.uploaded is
// committed only after the publish, so a crash re-runs the idempotent job
// and the event is sent at least once.
type Publisher struct {
	producer Producer
	source   string
}

// NewPublisher returns a Publisher.
func NewPublisher(p Producer, source string) *Publisher {
	return &Publisher{producer: p, source: source}
}

// Transcoded publishes track.transcoded.
func (p *Publisher) Transcoded(ctx context.Context, e mediav1.TrackTranscoded) error {
	return p.publish(ctx, e.TrackID, mediav1.TypeTrackTranscoded, e.TranscodedAt, e)
}

// Failed publishes media.transcode.failed.
func (p *Publisher) Failed(ctx context.Context, e mediav1.TranscodeFailed) error {
	return p.publish(ctx, e.TrackID, mediav1.TypeTranscodeFailed, e.FailedAt, e)
}

func (p *Publisher) publish(ctx context.Context, trackID, typ string, at time.Time, payload any) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	env, err := events.New(typ, mediav1.Version, p.source, traceID, at, payload)
	if err != nil {
		return err
	}
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}
	// Keyed by track, like track.uploaded: per-track order is kept.
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicMediaEvents, Key: []byte(trackID), Value: value,
		Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(mediav1.Version), "content-type": "application/json"},
	})
}
