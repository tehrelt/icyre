// Package domain holds the Analytics Worker's model: playback events as
// stored in ClickHouse and the metrics derived from them.
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
)

// Event is one playback event, enriched with the track's album and artists.
type Event struct {
	EventID    uuid.UUID
	Type       string
	PlaybackID uuid.UUID
	UserID     uuid.UUID
	TrackID    uuid.UUID
	AlbumID    uuid.UUID   // uuid.Nil when unknown
	ArtistIDs  []uuid.UUID // empty when unknown
	Source     string
	DurationMs int64
	ListenedMs int64
	At         time.Time
}

// ErrUnknownType marks an event type analytics does not count; such events
// are skipped, not dead-lettered.
var ErrUnknownType = errors.New("unknown event type")

// FromEnvelope decodes a playback.events message. The album and artists are
// filled in later (Enrich).
func FromEnvelope(env events.Envelope) (Event, error) {
	switch env.EventType {
	case playbackv1.TypeStarted, playbackv1.TypeFinished, playbackv1.TypeSkipped:
	default:
		return Event{}, fmt.Errorf("%w: %q", ErrUnknownType, env.EventType)
	}
	if env.EventVersion != playbackv1.Version {
		return Event{}, fmt.Errorf("unsupported %s version %d", env.EventType, env.EventVersion)
	}
	var p playbackv1.Playback
	if err := env.DecodePayload(&p); err != nil {
		return Event{}, fmt.Errorf("payload: %w", err)
	}
	eventID, e1 := uuid.Parse(env.EventID)
	playbackID, e2 := uuid.Parse(p.PlaybackID)
	userID, e3 := uuid.Parse(p.UserID)
	trackID, e4 := uuid.Parse(p.TrackID)
	if err := errors.Join(e1, e2, e3, e4); err != nil {
		return Event{}, fmt.Errorf("malformed IDs: %w", err)
	}
	if p.At.IsZero() || p.DurationMs < 0 || p.ListenedMs < 0 {
		return Event{}, errors.New("missing time or negative durations")
	}
	return Event{
		EventID: eventID, Type: env.EventType, PlaybackID: playbackID, UserID: userID, TrackID: trackID,
		Source: p.Source, DurationMs: p.DurationMs, ListenedMs: p.ListenedMs, At: p.At.UTC(),
	}, nil
}

// TrackInfo is what analytics needs from Catalog about a track.
type TrackInfo struct {
	AlbumID   uuid.UUID
	ArtistIDs []uuid.UUID
}

// Enrich returns a copy of e with the track's album and artists.
func (e Event) Enrich(t TrackInfo) Event {
	e.AlbumID = t.AlbumID
	e.ArtistIDs = t.ArtistIDs
	return e
}
