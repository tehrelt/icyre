// Package kafka publishes Catalog domain events as versioned contracts.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/catalogv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

// Producer is the part of the platform producer the publisher needs.
type Producer interface {
	Publish(ctx context.Context, msgs ...platformkafka.Message) error
}

// Publisher implements ports.EventPublisher on top of Kafka.
type Publisher struct {
	producer Producer
	source   string
	now      func() time.Time
}

// NewPublisher returns a Publisher that stamps events with source as producer.
func NewPublisher(p Producer, source string) *Publisher {
	return &Publisher{producer: p, source: source, now: time.Now}
}

// Publish maps each domain event to its contract and sends them in one batch.
// The aggregate ID is the partition key, preserving per-aggregate order.
func (p *Publisher) Publish(ctx context.Context, evs ...domain.Event) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}

	msgs := make([]platformkafka.Message, 0, len(evs))
	for _, ev := range evs {
		m, err := toMessage(ev)
		if err != nil {
			return err
		}
		env, err := events.New(m.eventType, catalogv1.Version, p.source, traceID, m.occurredAt, m.payload)
		if err != nil {
			return err
		}
		value, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("marshal envelope: %w", err)
		}
		msgs = append(msgs, platformkafka.Message{
			Topic: events.TopicCatalogEvents,
			Key:   []byte(m.key),
			Value: value,
			Headers: map[string]string{
				"event-type":    m.eventType,
				"event-version": fmt.Sprint(catalogv1.Version),
				"content-type":  "application/json",
			},
		})
	}
	return p.producer.Publish(ctx, msgs...)
}

type mapped struct {
	eventType  string
	key        string
	occurredAt time.Time
	payload    any
}

// toMessage is the single place where domain events meet wire contracts.
func toMessage(ev domain.Event) (mapped, error) {
	switch e := ev.(type) {
	case domain.ArtistCreated:
		return mapped{catalogv1.TypeArtistCreated, e.Artist.ID.String(), e.Artist.CreatedAt,
			catalogv1.Artist{ArtistID: e.Artist.ID.String(), Name: e.Artist.Name}}, nil
	case domain.AlbumCreated:
		return mapped{catalogv1.TypeAlbumCreated, e.Album.ID.String(), e.Album.CreatedAt, albumPayload(e.Album)}, nil
	case domain.TrackCreated:
		return mapped{catalogv1.TypeTrackCreated, e.Track.ID.String(), e.Track.CreatedAt, trackPayload(e.Track)}, nil
	case domain.TrackUpdated:
		return mapped{catalogv1.TypeTrackUpdated, e.Track.ID.String(), e.Track.UpdatedAt, trackPayload(e.Track)}, nil
	default:
		return mapped{}, fmt.Errorf("unsupported catalog event %T", ev)
	}
}

func albumPayload(a domain.Album) catalogv1.Album {
	return catalogv1.Album{
		AlbumID:     a.ID.String(),
		ArtistIDs:   ids(a.ArtistIDs),
		Title:       a.Title,
		AlbumType:   string(a.Type),
		ReleaseDate: a.ReleaseDate.Format(time.DateOnly),
		GenreIDs:    ids(a.GenreIDs),
	}
}

func trackPayload(t domain.Track) catalogv1.Track {
	return catalogv1.Track{
		TrackID:     t.ID.String(),
		AlbumID:     t.AlbumID.String(),
		ArtistIDs:   ids(t.ArtistIDs),
		Title:       t.Title,
		DurationMs:  t.Duration.Milliseconds(),
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
		Explicit:    t.Explicit,
		ISRC:        t.ISRC,
		Status:      string(t.Status),
		UpdatedAt:   t.UpdatedAt,
	}
}

func ids(in []uuid.UUID) []string {
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = id.String()
	}
	return out
}

// NopPublisher discards events; used when Kafka is disabled locally.
type NopPublisher struct{}

// Publish implements ports.EventPublisher.
func (NopPublisher) Publish(context.Context, ...domain.Event) error { return nil }
