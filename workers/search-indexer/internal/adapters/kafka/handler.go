// Package kafka turns catalog.events and playlist.events into index updates.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/catalogv1"
	"github.com/tehrelt/icyre/libs/contracts/events/playlistv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

// Indexer is the use case the consumer drives.
type Indexer interface {
	TrackChanged(ctx context.Context, t catalogv1.Track, at time.Time) error
	AlbumCreated(ctx context.Context, a catalogv1.Album, at time.Time) error
	ArtistCreated(ctx context.Context, a catalogv1.Artist, at time.Time) error
	PlaylistChanged(ctx context.Context, id string, at time.Time) error
	PlaylistDeleted(ctx context.Context, id string, at time.Time) error
}

// Handler returns the catalog.events and playlist.events handler. The envelope's occurredAt
// versions every write, so redelivered or reordered events are harmless
// (idempotent by document ID + external version).
func Handler(x Indexer) platformkafka.Handler {
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		if strings.HasPrefix(env.EventType, "playlist.") {
			return playlist(ctx, x, env)
		}
		if env.EventVersion != catalogv1.Version {
			return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
		}
		switch env.EventType {
		case catalogv1.TypeTrackCreated, catalogv1.TypeTrackUpdated:
			var p catalogv1.Track
			if err := env.DecodePayload(&p); err != nil {
				return permanent(err)
			}
			return classify(x.TrackChanged(ctx, p, env.OccurredAt))
		case catalogv1.TypeAlbumCreated:
			var p catalogv1.Album
			if err := env.DecodePayload(&p); err != nil {
				return permanent(err)
			}
			return classify(x.AlbumCreated(ctx, p, env.OccurredAt))
		case catalogv1.TypeArtistCreated:
			var p catalogv1.Artist
			if err := env.DecodePayload(&p); err != nil {
				return permanent(err)
			}
			return classify(x.ArtistCreated(ctx, p, env.OccurredAt))
		}
		return nil // other catalog events do not affect search
	}
}

// playlist handles playlist.events. Every payload names the playlist; the
// indexer reads its current state, so all changes but a delete are handled
// alike.
func playlist(ctx context.Context, x Indexer, env events.Envelope) error {
	if env.EventVersion != playlistv1.Version {
		return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
	}
	var p struct {
		PlaylistID string `json:"playlistId"`
	}
	if err := env.DecodePayload(&p); err != nil {
		return permanent(err)
	}
	if p.PlaylistID == "" {
		return permanent(errors.New("playlistId is empty"))
	}
	if env.EventType == playlistv1.TypeDeleted {
		return x.PlaylistDeleted(ctx, p.PlaylistID, env.OccurredAt)
	}
	return x.PlaylistChanged(ctx, p.PlaylistID, env.OccurredAt)
}

func permanent(err error) error {
	return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
}

// classify: a reference Catalog does not know will not appear on retry.
func classify(err error) error {
	if errors.Is(err, application.ErrNotFound) {
		return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
	}
	return err
}
