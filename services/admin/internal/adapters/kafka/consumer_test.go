package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

type stubRecorder struct {
	jobs []domain.TranscodeJob
	err  error
}

func (s *stubRecorder) RecordTranscodeEvent(_ context.Context, job domain.TranscodeJob) error {
	if s.err != nil {
		return s.err
	}
	s.jobs = append(s.jobs, job)
	return nil
}

func encodeEnvelope(t *testing.T, eventType string, version int, payload any) []byte {
	t.Helper()
	env, err := events.New(eventType, version, "media-ingest", "", time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMediaEventsHandler(t *testing.T) {
	rec := &stubRecorder{}
	h := MediaEventsHandler(rec, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	uploadID := uuid.New()
	trackID := uuid.New()
	ts := time.Date(2026, 11, 22, 11, 0, 0, 0, time.UTC)

	// 1. track.uploaded -> QUEUED
	err := h(ctx, platformkafka.Record{Value: encodeEnvelope(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{
		UploadID: uploadID.String(), TrackID: trackID.String(), UploadedAt: ts,
	})})
	if err != nil || len(rec.jobs) != 1 || rec.jobs[0].Status != domain.TranscodeQueued {
		t.Fatalf("uploaded: jobs=%+v err=%v", rec.jobs, err)
	}

	// 2. track.transcoded -> SUCCEEDED
	err = h(ctx, platformkafka.Record{Value: encodeEnvelope(t, mediav1.TypeTrackTranscoded, mediav1.Version, mediav1.TrackTranscoded{
		UploadID: uploadID.String(), TrackID: trackID.String(), TranscodedAt: ts.Add(time.Minute),
	})})
	if err != nil || len(rec.jobs) != 2 || rec.jobs[1].Status != domain.TranscodeSucceeded {
		t.Fatalf("transcoded: jobs=%+v err=%v", rec.jobs, err)
	}

	// 3. media.transcode.failed -> FAILED
	err = h(ctx, platformkafka.Record{Value: encodeEnvelope(t, mediav1.TypeTranscodeFailed, mediav1.Version, mediav1.TranscodeFailed{
		UploadID: uploadID.String(), TrackID: trackID.String(), Reason: mediav1.ReasonUndecodable, FailedAt: ts.Add(2 * time.Minute),
	})})
	if err != nil || len(rec.jobs) != 3 || rec.jobs[2].Status != domain.TranscodeFailed || rec.jobs[2].FailureReason != mediav1.ReasonUndecodable {
		t.Fatalf("failed: jobs=%+v err=%v", rec.jobs, err)
	}

	// 4. Unrelated event skipped.
	err = h(ctx, platformkafka.Record{Value: encodeEnvelope(t, mediav1.TypeIngestFailed, mediav1.Version, mediav1.IngestFailed{
		UploadID: uploadID.String(), TrackID: trackID.String(), Reason: mediav1.ReasonExpired, FailedAt: ts,
	})})
	if err != nil || len(rec.jobs) != 3 {
		t.Fatalf("expected skip, got jobs=%d err=%v", len(rec.jobs), err)
	}

	// 5. Permanent errors: bad JSON, bad version, bad UUID.
	for _, bad := range [][]byte{
		[]byte(`{not-json`),
		encodeEnvelope(t, mediav1.TypeTrackUploaded, 99, mediav1.TrackUploaded{UploadID: uploadID.String(), TrackID: trackID.String()}),
		encodeEnvelope(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{UploadID: "bad-uuid", TrackID: trackID.String()}),
		encodeEnvelope(t, mediav1.TypeTrackTranscoded, mediav1.Version, mediav1.TrackTranscoded{UploadID: uploadID.String(), TrackID: "bad-uuid"}),
	} {
		if err := h(ctx, platformkafka.Record{Value: bad}); !errors.Is(err, platformkafka.ErrPermanent) {
			t.Fatalf("expected ErrPermanent, got %v", err)
		}
	}

	// 6. Transient DB error is not ErrPermanent.
	rec.err = errors.New("db down")
	err = h(ctx, platformkafka.Record{Value: encodeEnvelope(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{
		UploadID: uploadID.String(), TrackID: trackID.String(), UploadedAt: ts,
	})})
	if err == nil || errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("expected transient error, got %v", err)
	}
}
