package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

// MediaPipeline is the use case the media.events consumer drives.
type MediaPipeline interface {
	AdvanceTrackMedia(ctx context.Context, id uuid.UUID, stage domain.MediaStage) (domain.Track, error)
}

// mediaStages maps the media.events types Catalog reacts to. The rest
// (media.ingest.failed: the track never left DRAFT) are skipped.
var mediaStages = map[string]domain.MediaStage{
	mediav1.TypeTrackUploaded:   domain.MediaUploaded,
	mediav1.TypeTrackTranscoded: domain.MediaTranscoded,
	mediav1.TypeTranscodeFailed: domain.MediaFailed,
}

// MediaEventsHandler returns the handler for media.events: track.uploaded,
// track.transcoded and media.transcode.failed move the track status.
// Undecodable messages are permanent failures (straight to the DLQ); events
// about tracks Catalog does not know are logged and acknowledged.
func MediaEventsHandler(app MediaPipeline, log *slog.Logger) platformkafka.Handler {
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		stage, ok := mediaStages[env.EventType]
		if !ok {
			return nil
		}
		if env.EventVersion != mediav1.Version {
			return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
		}
		// Every media payload carries trackId.
		var p struct {
			TrackID string `json:"trackId"`
		}
		if err := env.DecodePayload(&p); err != nil {
			return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}
		id, err := uuid.Parse(p.TrackID)
		if err != nil {
			return fmt.Errorf("%w: trackId: %v", platformkafka.ErrPermanent, err)
		}
		t, err := app.AdvanceTrackMedia(ctx, id, stage)
		if errors.Is(err, domain.ErrTrackNotFound) {
			log.WarnContext(ctx, "media event for unknown track skipped", "event_type", env.EventType, "track_id", id)
			return nil
		}
		if err != nil {
			return err
		}
		log.DebugContext(ctx, "media event applied", "event_type", env.EventType, "track_id", id, "status", t.Status)
		return nil
	}
}
