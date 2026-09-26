// Package kafka publishes playlist.events.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playlistv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/playlist/internal/domain"
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

func (p *Publisher) publish(ctx context.Context, playlistID uuid.UUID, typ string, at time.Time, payload any) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}
	env, err := events.New(typ, playlistv1.Version, p.source, traceID, at, payload)
	if err != nil {
		return err
	}
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}
	// Keyed by playlist: one playlist's changes stay in order.
	return p.producer.Publish(ctx, platformkafka.Message{
		Topic: events.TopicPlaylistEvents, Key: []byte(playlistID.String()), Value: value,
		Headers: map[string]string{"event-type": typ, "event-version": fmt.Sprint(playlistv1.Version), "content-type": "application/json"},
	})
}

// Created publishes playlist.created.
func (p *Publisher) Created(ctx context.Context, pl domain.Playlist) error {
	return p.publish(ctx, pl.ID, playlistv1.TypeCreated, pl.CreatedAt, playlistv1.Created{
		PlaylistID: pl.ID.String(), OwnerID: pl.OwnerID.String(), Title: pl.Title, CreatedAt: pl.CreatedAt,
	})
}

// Updated publishes playlist.updated.
func (p *Publisher) Updated(ctx context.Context, pl domain.Playlist) error {
	return p.publish(ctx, pl.ID, playlistv1.TypeUpdated, pl.UpdatedAt, playlistv1.Updated{
		PlaylistID: pl.ID.String(), OwnerID: pl.OwnerID.String(), Title: pl.Title, UpdatedAt: pl.UpdatedAt,
	})
}

// Deleted publishes playlist.deleted.
func (p *Publisher) Deleted(ctx context.Context, pl domain.Playlist, at time.Time) error {
	return p.publish(ctx, pl.ID, playlistv1.TypeDeleted, at, playlistv1.Deleted{
		PlaylistID: pl.ID.String(), OwnerID: pl.OwnerID.String(), DeletedAt: at,
	})
}

// TrackAdded publishes playlist.track_added.
func (p *Publisher) TrackAdded(ctx context.Context, pl domain.Playlist, t domain.Track) error {
	return p.publish(ctx, pl.ID, playlistv1.TypeTrackAdded, t.AddedAt, playlistv1.TrackAdded{
		PlaylistID: pl.ID.String(), OwnerID: pl.OwnerID.String(), TrackID: t.TrackID.String(), Position: t.Position, AddedAt: t.AddedAt,
	})
}

// TrackRemoved publishes playlist.track_removed.
func (p *Publisher) TrackRemoved(ctx context.Context, pl domain.Playlist, trackID uuid.UUID, at time.Time) error {
	return p.publish(ctx, pl.ID, playlistv1.TypeTrackRemoved, at, playlistv1.TrackRemoved{
		PlaylistID: pl.ID.String(), OwnerID: pl.OwnerID.String(), TrackID: trackID.String(), RemovedAt: at,
	})
}

// TracksReordered publishes playlist.tracks_reordered.
func (p *Publisher) TracksReordered(ctx context.Context, pl domain.Playlist, order []uuid.UUID, at time.Time) error {
	ids := make([]string, len(order))
	for i, id := range order {
		ids[i] = id.String()
	}
	return p.publish(ctx, pl.ID, playlistv1.TypeTracksReordered, at, playlistv1.TracksReordered{
		PlaylistID: pl.ID.String(), OwnerID: pl.OwnerID.String(), TrackIDs: ids, ReorderedAt: at,
	})
}

// NopPublisher discards events.
type NopPublisher struct{}

// Created implements application.Publisher.
func (NopPublisher) Created(context.Context, domain.Playlist) error { return nil }

// Updated implements application.Publisher.
func (NopPublisher) Updated(context.Context, domain.Playlist) error { return nil }

// Deleted implements application.Publisher.
func (NopPublisher) Deleted(context.Context, domain.Playlist, time.Time) error { return nil }

// TrackAdded implements application.Publisher.
func (NopPublisher) TrackAdded(context.Context, domain.Playlist, domain.Track) error { return nil }

// TrackRemoved implements application.Publisher.
func (NopPublisher) TrackRemoved(context.Context, domain.Playlist, uuid.UUID, time.Time) error {
	return nil
}

// TracksReordered implements application.Publisher.
func (NopPublisher) TracksReordered(context.Context, domain.Playlist, []uuid.UUID, time.Time) error {
	return nil
}
