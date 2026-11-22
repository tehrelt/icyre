// Package kafka consumes media pipeline events for the Admin Service.
package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// TranscodeRecorder projects transcode job events into the admin read model.
type TranscodeRecorder interface {
	RecordTranscodeEvent(ctx context.Context, job domain.TranscodeJob) error
}

// MediaEventsHandler returns a Kafka handler for media.events.
func MediaEventsHandler(app TranscodeRecorder, log *slog.Logger) platformkafka.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		job, ok, err := decodeTranscodeJob(env)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := app.RecordTranscodeEvent(ctx, job); err != nil {
			return err
		}
		log.DebugContext(ctx, "transcode job projected",
			"event_type", env.EventType, "upload_id", job.UploadID, "track_id", job.TrackID, "status", job.Status)
		return nil
	}
}

func decodeTranscodeJob(env events.Envelope) (domain.TranscodeJob, bool, error) {
	switch env.EventType {
	case mediav1.TypeTrackUploaded:
		if err := checkVersion(env); err != nil {
			return domain.TranscodeJob{}, false, err
		}
		var p mediav1.TrackUploaded
		if err := env.DecodePayload(&p); err != nil {
			return domain.TranscodeJob{}, false, fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}
		return buildJob(p.UploadID, p.TrackID, domain.TranscodeQueued, "", p.UploadedAt, env.OccurredAt)

	case mediav1.TypeTrackTranscoded:
		if err := checkVersion(env); err != nil {
			return domain.TranscodeJob{}, false, err
		}
		var p mediav1.TrackTranscoded
		if err := env.DecodePayload(&p); err != nil {
			return domain.TranscodeJob{}, false, fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}
		return buildJob(p.UploadID, p.TrackID, domain.TranscodeSucceeded, "", p.TranscodedAt, env.OccurredAt)

	case mediav1.TypeTranscodeFailed:
		if err := checkVersion(env); err != nil {
			return domain.TranscodeJob{}, false, err
		}
		var p mediav1.TranscodeFailed
		if err := env.DecodePayload(&p); err != nil {
			return domain.TranscodeJob{}, false, fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}
		return buildJob(p.UploadID, p.TrackID, domain.TranscodeFailed, p.Reason, p.FailedAt, env.OccurredAt)

	default:
		return domain.TranscodeJob{}, false, nil
	}
}

func checkVersion(env events.Envelope) error {
	if env.EventVersion != mediav1.Version {
		return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
	}
	return nil
}

func buildJob(rawUploadID, rawTrackID string, status domain.TranscodeStatus, reason string, eventAt, fallbackAt time.Time) (domain.TranscodeJob, bool, error) {
	uploadID, err := uuid.Parse(rawUploadID)
	if err != nil || uploadID == uuid.Nil {
		return domain.TranscodeJob{}, false, fmt.Errorf("%w: invalid uploadId", platformkafka.ErrPermanent)
	}
	trackID, err := uuid.Parse(rawTrackID)
	if err != nil || trackID == uuid.Nil {
		return domain.TranscodeJob{}, false, fmt.Errorf("%w: invalid trackId", platformkafka.ErrPermanent)
	}
	if eventAt.IsZero() {
		eventAt = fallbackAt
	}
	return domain.TranscodeJob{
		UploadID:      uploadID,
		TrackID:       trackID,
		Status:        status,
		FailureReason: reason,
		EventAt:       eventAt.UTC(),
	}, true, nil
}
