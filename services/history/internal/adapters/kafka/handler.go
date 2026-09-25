// Package kafka turns playback.events into listens.
package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/history/internal/domain"
)

// Recorder is the use case the consumer drives.
type Recorder interface {
	PlayEnded(ctx context.Context, l domain.Listen) (bool, error)
}

// Handler consumes playback.finished and playback.skipped; playback.started
// carries nothing to record. Idempotent: the playback ID is the key.
func Handler(r Recorder, log *slog.Logger) platformkafka.Handler {
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		if env.EventType != playbackv1.TypeFinished && env.EventType != playbackv1.TypeSkipped {
			return nil
		}
		if env.EventVersion != playbackv1.Version {
			return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
		}
		var p playbackv1.Playback
		if err := env.DecodePayload(&p); err != nil {
			return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}
		playbackID, e1 := uuid.Parse(p.PlaybackID)
		userID, e2 := uuid.Parse(p.UserID)
		trackID, e3 := uuid.Parse(p.TrackID)
		if e1 != nil || e2 != nil || e3 != nil {
			return fmt.Errorf("%w: malformed IDs", platformkafka.ErrPermanent)
		}
		recorded, err := r.PlayEnded(ctx, domain.Listen{
			PlaybackID: playbackID, UserID: userID, TrackID: trackID, Source: p.Source,
			DurationMs: p.DurationMs, ListenedMs: p.ListenedMs, PlayedAt: p.At,
		})
		if recorded {
			log.DebugContext(ctx, "listen recorded", "track_id", p.TrackID)
		}
		return err
	}
}
