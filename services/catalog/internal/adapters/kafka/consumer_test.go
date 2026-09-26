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
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

type pipelineCall struct {
	id    uuid.UUID
	stage domain.MediaStage
}

type fakePipeline struct {
	calls []pipelineCall
	err   error
}

func (f *fakePipeline) AdvanceTrackMedia(_ context.Context, id uuid.UUID, stage domain.MediaStage) (domain.Track, error) {
	f.calls = append(f.calls, pipelineCall{id, stage})
	return domain.Track{ID: id}, f.err
}

func mediaRecord(t *testing.T, typ string, version int, payload any) platformkafka.Record {
	t.Helper()
	env, err := events.New(typ, version, "transcoder", "", time.Now(), payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	return platformkafka.Record{Topic: events.TopicMediaEvents, Value: raw}
}

func TestMediaEventsHandlerMapsStages(t *testing.T) {
	app := &fakePipeline{}
	h := MediaEventsHandler(app, slog.New(slog.NewTextHandler(io.Discard, nil)))
	id := uuid.New()
	ctx := context.Background()

	recs := []platformkafka.Record{
		mediaRecord(t, mediav1.TypeTrackUploaded, 1, mediav1.TrackUploaded{TrackID: id.String()}),
		mediaRecord(t, mediav1.TypeTrackTranscoded, 1, mediav1.TrackTranscoded{TrackID: id.String()}),
		mediaRecord(t, mediav1.TypeTranscodeFailed, 1, mediav1.TranscodeFailed{TrackID: id.String()}),
		mediaRecord(t, mediav1.TypeIngestFailed, 1, mediav1.IngestFailed{TrackID: id.String()}), // skipped
	}
	for _, r := range recs {
		if err := h(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	want := []pipelineCall{{id, domain.MediaUploaded}, {id, domain.MediaTranscoded}, {id, domain.MediaFailed}}
	if len(app.calls) != len(want) {
		t.Fatalf("calls = %v", app.calls)
	}
	for i := range want {
		if app.calls[i] != want[i] {
			t.Fatalf("call %d = %v, want %v", i, app.calls[i], want[i])
		}
	}
}

func TestMediaEventsHandlerErrors(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	id := uuid.New().String()

	permanent := []platformkafka.Record{
		{Value: []byte("garbage")},
		mediaRecord(t, mediav1.TypeTrackTranscoded, 2, mediav1.TrackTranscoded{TrackID: id}),
		mediaRecord(t, mediav1.TypeTrackTranscoded, 1, mediav1.TrackTranscoded{TrackID: "nope"}),
	}
	for i, r := range permanent {
		if err := MediaEventsHandler(&fakePipeline{}, log)(ctx, r); !errors.Is(err, platformkafka.ErrPermanent) {
			t.Fatalf("record %d: want permanent, got %v", i, err)
		}
	}

	rec := mediaRecord(t, mediav1.TypeTrackUploaded, 1, mediav1.TrackUploaded{TrackID: id})
	if err := MediaEventsHandler(&fakePipeline{err: domain.ErrTrackNotFound}, log)(ctx, rec); err != nil {
		t.Fatalf("unknown track must be acknowledged: %v", err)
	}
	transient := errors.New("db down")
	if err := MediaEventsHandler(&fakePipeline{err: transient}, log)(ctx, rec); !errors.Is(err, transient) || errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("storage errors must be retried: %v", err)
	}
}
