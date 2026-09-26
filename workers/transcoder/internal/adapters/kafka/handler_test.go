package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
)

type fake struct {
	jobs     []mediav1.TrackUploaded
	err      error
	deadline bool
}

func (f *fake) Handle(ctx context.Context, job mediav1.TrackUploaded) (application.Outcome, error) {
	_, f.deadline = ctx.Deadline()
	f.jobs = append(f.jobs, job)
	return application.OutcomeTranscoded, f.err
}

func record(t *testing.T, typ string, version int, payload any) platformkafka.Record {
	t.Helper()
	env, err := events.New(typ, version, "media-ingest", "", time.Now(), payload)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(env)
	return platformkafka.Record{Topic: events.TopicMediaEvents, Value: b}
}

func handler(f *fake) platformkafka.Handler {
	return Handler(f, NewMetrics(prometheus.NewRegistry()), time.Minute)
}

func TestHandlesTrackUploaded(t *testing.T) {
	f := &fake{}
	err := handler(f)(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{TrackID: "t1", UploadID: "u1"}))
	if err != nil || len(f.jobs) != 1 || f.jobs[0].TrackID != "t1" || !f.deadline {
		t.Fatal(err, f.jobs, f.deadline)
	}
}

func TestIgnoresOtherMediaEvents(t *testing.T) {
	f := &fake{}
	h := handler(f)
	for _, typ := range []string{mediav1.TypeTrackTranscoded, mediav1.TypeTranscodeFailed, mediav1.TypeIngestFailed} {
		if err := h(context.Background(), record(t, typ, mediav1.Version, map[string]string{"trackId": "t1"})); err != nil {
			t.Fatal(typ, err)
		}
	}
	if len(f.jobs) != 0 {
		t.Fatal(f.jobs)
	}
}

func TestPermanentFailures(t *testing.T) {
	h := handler(&fake{err: application.ErrInvalidJob})
	for name, rec := range map[string]platformkafka.Record{
		"garbage":     {Value: []byte("{")},
		"version":     record(t, mediav1.TypeTrackUploaded, 99, mediav1.TrackUploaded{}),
		"payload":     record(t, mediav1.TypeTrackUploaded, mediav1.Version, "nope"),
		"invalid job": record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{}),
	} {
		if err := h(context.Background(), rec); !errors.Is(err, platformkafka.ErrPermanent) {
			t.Fatal(name, err)
		}
	}
}

func TestTransientFailureIsRetried(t *testing.T) {
	boom := errors.New("storage down")
	err := handler(&fake{err: boom})(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{}))
	if !errors.Is(err, boom) || errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatal(err)
	}
}
