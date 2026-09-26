package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/metadata/internal/application"
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

type fakeExtractor struct {
	jobs []mediav1.TrackUploaded
	err  error
}

func (f *fakeExtractor) Handle(_ context.Context, job mediav1.TrackUploaded) (application.Outcome, error) {
	f.jobs = append(f.jobs, job)
	return application.Extracted, f.err
}

func record(t *testing.T, typ string, version int, payload any) platformkafka.Record {
	t.Helper()
	env, err := events.New(typ, version, "media-ingest", "", time.Now(), payload)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := json.Marshal(env)
	return platformkafka.Record{Value: value}
}

func handler(x Extractor) platformkafka.Handler {
	return Handler(x, NewMetrics(prometheus.NewRegistry()), time.Second)
}

func TestHandlerDispatchesTrackUploaded(t *testing.T) {
	x := &fakeExtractor{}
	job := mediav1.TrackUploaded{UploadID: uuid.NewString(), TrackID: uuid.NewString(), Key: "k"}
	if err := handler(x)(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, job)); err != nil {
		t.Fatal(err)
	}
	if len(x.jobs) != 1 || x.jobs[0].UploadID != job.UploadID {
		t.Fatalf("jobs = %+v", x.jobs)
	}
}

func TestHandlerIgnoresOtherEvents(t *testing.T) {
	x := &fakeExtractor{}
	rec := record(t, mediav1.TypeMetadataExtracted, mediav1.Version, mediav1.MetadataExtracted{})
	if err := handler(x)(context.Background(), rec); err != nil || len(x.jobs) != 0 {
		t.Fatalf("err = %v, jobs = %d", err, len(x.jobs))
	}
}

func TestHandlerPermanentErrors(t *testing.T) {
	for name, rec := range map[string]platformkafka.Record{
		"garbage":     {Value: []byte("{")},
		"new version": record(t, mediav1.TypeTrackUploaded, mediav1.Version+1, mediav1.TrackUploaded{}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := handler(&fakeExtractor{})(context.Background(), rec); !errors.Is(err, platformkafka.ErrPermanent) {
				t.Fatalf("err = %v, want ErrPermanent", err)
			}
		})
	}
	x := &fakeExtractor{err: application.ErrInvalidJob}
	if err := handler(x)(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{})); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("invalid job: err = %v, want ErrPermanent", err)
	}
}

func TestHandlerRetryableError(t *testing.T) {
	boom := errors.New("boom")
	err := handler(&fakeExtractor{err: boom})(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{}))
	if !errors.Is(err, boom) || errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("err = %v, want retryable boom", err)
	}
}

func TestEncoder(t *testing.T) {
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	m := domain.Metadata{
		UploadID: uuid.New(), TrackID: uuid.New(), SourceSHA256: "ab12",
		Probe:      domain.Probe{Container: "flac", Codec: "flac", DurationMs: 1000, BitrateBps: 900000, SampleRateHz: 44100, Channels: 2},
		UploadedAt: at, ExtractedAt: at.Add(time.Second),
	}
	msg, err := Encoder("metadata-worker")(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Topic != events.TopicMediaEvents || string(msg.Key) != m.TrackID.String() || msg.Headers["event-type"] != mediav1.TypeMetadataExtracted {
		t.Fatalf("message = %+v", msg)
	}
	env, err := events.Decode(msg.Value)
	if err != nil {
		t.Fatal(err)
	}
	var p mediav1.MetadataExtracted
	if err := env.DecodePayload(&p); err != nil {
		t.Fatal(err)
	}
	want := mediav1.MetadataExtracted{
		UploadID: m.UploadID.String(), TrackID: m.TrackID.String(), Container: "flac", Codec: "flac", DurationMs: 1000,
		BitrateBps: 900000, SampleRateHz: 44100, Channels: 2, SourceSHA256: "ab12", UploadedAt: at, ExtractedAt: at.Add(time.Second),
	}
	if p != want || env.EventVersion != mediav1.Version {
		t.Fatalf("payload = %+v, want %+v", p, want)
	}
}
