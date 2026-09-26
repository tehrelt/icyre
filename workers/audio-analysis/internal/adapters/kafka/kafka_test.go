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
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/application"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

type fakeAnalyzer struct {
	jobs []mediav1.TrackUploaded
	err  error
}

func (f *fakeAnalyzer) Handle(_ context.Context, job mediav1.TrackUploaded) (application.Outcome, error) {
	f.jobs = append(f.jobs, job)
	return application.Analyzed, f.err
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

func handler(x Analyzer) platformkafka.Handler {
	return Handler(x, NewMetrics(prometheus.NewRegistry()), time.Second)
}

func TestHandlerDispatchesTrackUploaded(t *testing.T) {
	x := &fakeAnalyzer{}
	job := mediav1.TrackUploaded{UploadID: uuid.NewString(), TrackID: uuid.NewString(), Key: "k"}
	if err := handler(x)(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, job)); err != nil {
		t.Fatal(err)
	}
	if len(x.jobs) != 1 || x.jobs[0].UploadID != job.UploadID {
		t.Fatalf("jobs = %+v", x.jobs)
	}
}

func TestHandlerIgnoresOtherEvents(t *testing.T) {
	x := &fakeAnalyzer{}
	rec := record(t, mediav1.TypeAudioFeaturesExtracted, mediav1.Version, mediav1.AudioFeaturesExtracted{})
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
			if err := handler(&fakeAnalyzer{})(context.Background(), rec); !errors.Is(err, platformkafka.ErrPermanent) {
				t.Fatalf("err = %v, want ErrPermanent", err)
			}
		})
	}
	x := &fakeAnalyzer{err: application.ErrInvalidJob}
	if err := handler(x)(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{})); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("invalid job: err = %v, want ErrPermanent", err)
	}
}

func TestHandlerRetryableError(t *testing.T) {
	boom := errors.New("boom")
	err := handler(&fakeAnalyzer{err: boom})(context.Background(), record(t, mediav1.TypeTrackUploaded, mediav1.Version, mediav1.TrackUploaded{}))
	if !errors.Is(err, boom) || errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("err = %v, want retryable boom", err)
	}
}

func TestEncoder(t *testing.T) {
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	a := domain.Analysis{
		UploadID: uuid.New(), TrackID: uuid.New(), SourceSHA256: "ab12", AnalyzerVersion: domain.AnalyzerVersion,
		Features: domain.Features{
			BPM: 128, BPMConfidence: 0.71, IntegratedLUFS: -9.4, LoudnessRangeLU: 5.2, TruePeakDBTP: -0.3, SilenceRatio: 0.02, AnalyzedMs: 215000,
		},
		UploadedAt: at, AnalyzedAt: at.Add(time.Second),
	}
	msg, err := Encoder("audio-analysis-worker")(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Topic != events.TopicMediaEvents || string(msg.Key) != a.TrackID.String() || msg.Headers["event-type"] != mediav1.TypeAudioFeaturesExtracted {
		t.Fatalf("message = %+v", msg)
	}
	env, err := events.Decode(msg.Value)
	if err != nil {
		t.Fatal(err)
	}
	var p mediav1.AudioFeaturesExtracted
	if err := env.DecodePayload(&p); err != nil {
		t.Fatal(err)
	}
	if p.BPM == nil || *p.BPM != 128 || p.UploadID != a.UploadID.String() || p.TrackID != a.TrackID.String() ||
		p.BPMConfidence != 0.71 || p.IntegratedLUFS != -9.4 || p.LoudnessRangeLU != 5.2 || p.TruePeakDBTP != -0.3 ||
		p.SilenceRatio != 0.02 || p.AnalyzedMs != 215000 || p.AnalyzerVersion != "1" || p.SourceSHA256 != "ab12" ||
		!p.UploadedAt.Equal(at) || !p.AnalyzedAt.Equal(at.Add(time.Second)) || env.EventVersion != mediav1.Version {
		t.Fatalf("payload = %+v", p)
	}
}

func TestPayloadWithoutTempo(t *testing.T) {
	if p := Payload(domain.Analysis{Features: domain.Features{AnalyzedMs: 1000}}); p.BPM != nil {
		t.Fatalf("bpm = %v, want null", *p.BPM)
	}
}
