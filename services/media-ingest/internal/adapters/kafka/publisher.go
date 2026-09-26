// Package kafka publishes media.events.
package kafka

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

// Producer is the part of the platform producer used here.
type Producer interface {
	Publish(ctx context.Context, msgs ...platformkafka.Message) error
}

// Publisher implements application.Publisher.
type Publisher struct {
	producer Producer
	source   string
	bucket   string
}

// NewPublisher returns a Publisher; bucket is where uploads are stored.
func NewPublisher(p Producer, source, bucket string) *Publisher {
	return &Publisher{producer: p, source: source, bucket: bucket}
}

func (p *Publisher) publish(ctx context.Context, u domain.Upload, typ string, at time.Time, payload any) error {
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
	// Keyed by track: uploads of one track stay in order.
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicMediaEvents, Key: []byte(u.TrackID.String()), Value: value,
		Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(mediav1.Version), "content-type": "application/json"},
	})
}

func settledAt(u domain.Upload) time.Time {
	if u.CompletedAt != nil {
		return *u.CompletedAt
	}
	return time.Now().UTC()
}

// Uploaded publishes track.uploaded.
func (p *Publisher) Uploaded(ctx context.Context, u domain.Upload) error {
	at := settledAt(u)
	return p.publish(ctx, u, mediav1.TypeTrackUploaded, at, mediav1.TrackUploaded{
		UploadID: u.ID.String(), TrackID: u.TrackID.String(), UploaderID: u.UploaderID.String(),
		Bucket: p.bucket, Key: u.Key, ContentType: u.ContentType, SizeBytes: u.SizeBytes,
		SHA256: hex.EncodeToString(u.SHA256), UploadedAt: at,
	})
}

// Failed publishes media.ingest.failed.
func (p *Publisher) Failed(ctx context.Context, u domain.Upload) error {
	at := settledAt(u)
	return p.publish(ctx, u, mediav1.TypeIngestFailed, at, mediav1.IngestFailed{
		UploadID: u.ID.String(), TrackID: u.TrackID.String(), Reason: u.FailureReason, FailedAt: at,
	})
}

// NopPublisher discards events.
type NopPublisher struct{}

// Uploaded implements application.Publisher.
func (NopPublisher) Uploaded(context.Context, domain.Upload) error { return nil }

// Failed implements application.Publisher.
func (NopPublisher) Failed(context.Context, domain.Upload) error { return nil }
