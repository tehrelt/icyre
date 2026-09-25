// Package kafka publishes library.events.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/libraryv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/library/internal/domain"
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

func (p *Publisher) publish(ctx context.Context, it domain.Item, typ string, payload any) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	env, err := events.New(typ, libraryv1.Version, p.source, traceID, it.SavedAt, payload)
	if err != nil {
		return err
	}
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}
	// Keyed by user: one listener's saves and removals stay in order.
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicLibraryEvents, Key: []byte(it.UserID.String()), Value: value,
		Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(libraryv1.Version), "content-type": "application/json"},
	})
}

// Saved publishes library.track_saved / library.album_saved.
func (p *Publisher) Saved(ctx context.Context, it domain.Item) error {
	if it.Kind == domain.KindAlbum {
		return p.publish(ctx, it, libraryv1.TypeAlbumSaved, libraryv1.AlbumSaved{UserID: it.UserID.String(), AlbumID: it.EntityID.String(), SavedAt: it.SavedAt})
	}
	return p.publish(ctx, it, libraryv1.TypeTrackSaved, libraryv1.TrackSaved{UserID: it.UserID.String(), TrackID: it.EntityID.String(), SavedAt: it.SavedAt})
}

// Removed publishes library.track_removed / library.album_removed. For a
// removal Item.SavedAt carries the removal time.
func (p *Publisher) Removed(ctx context.Context, it domain.Item) error {
	if it.Kind == domain.KindAlbum {
		return p.publish(ctx, it, libraryv1.TypeAlbumRemoved, libraryv1.AlbumRemoved{UserID: it.UserID.String(), AlbumID: it.EntityID.String(), RemovedAt: it.SavedAt})
	}
	return p.publish(ctx, it, libraryv1.TypeTrackRemoved, libraryv1.TrackRemoved{UserID: it.UserID.String(), TrackID: it.EntityID.String(), RemovedAt: it.SavedAt})
}

// NopPublisher discards events.
type NopPublisher struct{}

// Saved implements application.Publisher.
func (NopPublisher) Saved(context.Context, domain.Item) error { return nil }

// Removed implements application.Publisher.
func (NopPublisher) Removed(context.Context, domain.Item) error { return nil }
