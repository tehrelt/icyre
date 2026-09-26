// Package kafka consumes track.uploaded and encodes audio.features_extracted.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/application"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

// Analyzer is the use case the consumer drives.
type Analyzer interface {
	Handle(ctx context.Context, job mediav1.TrackUploaded) (application.Outcome, error)
}

// Metrics are the worker's own metrics (specs/observability).
type Metrics struct {
	duration *prometheus.HistogramVec
}

// NewMetrics registers media_audio_analysis_duration_seconds.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "media_audio_analysis_duration_seconds",
		Help:    "Time to process one track.uploaded by result (analyzed, duplicate, missing, undecodable, error).",
		Buckets: []float64{0.05, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
	}, []string{"result"})}
	reg.MustRegister(m.duration)
	return m
}

// Handler returns the media.events handler. Only track.uploaded matters;
// the topic also carries this worker's own events. Every job runs under
// timeout.
func Handler(x Analyzer, m *Metrics, timeout time.Duration) platformkafka.Handler {
	return func(ctx context.Context, rec platformkafka.Record) error {
		env, err := events.Decode(rec.Value)
		if err != nil {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		if env.EventType != mediav1.TypeTrackUploaded {
			return nil
		}
		if env.EventVersion != mediav1.Version {
			return fmt.Errorf("%w: unsupported %s version %d", platformkafka.ErrPermanent, env.EventType, env.EventVersion)
		}
		var job mediav1.TrackUploaded
		if err := env.DecodePayload(&job); err != nil {
			return fmt.Errorf("%w: payload: %v", platformkafka.ErrPermanent, err)
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		start := time.Now()
		outcome, err := x.Handle(ctx, job)
		result := string(outcome)
		if err != nil {
			result = "error"
		}
		m.duration.WithLabelValues(result).Observe(time.Since(start).Seconds())
		if errors.Is(err, application.ErrInvalidJob) {
			return fmt.Errorf("%w: %v", platformkafka.ErrPermanent, err)
		}
		return err
	}
}

// Encoder returns the audio.features_extracted encoder; source names the
// producing service in the envelope.
func Encoder(source string) func(ctx context.Context, a domain.Analysis) (platformkafka.Message, error) {
	return func(ctx context.Context, a domain.Analysis) (platformkafka.Message, error) {
		traceID := ""
		if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
			traceID = sc.TraceID().String()
		}
		env, err := events.New(mediav1.TypeAudioFeaturesExtracted, mediav1.Version, source, traceID, a.AnalyzedAt, Payload(a))
		if err != nil {
			return platformkafka.Message{}, err
		}
		value, err := json.Marshal(env)
		if err != nil {
			return platformkafka.Message{}, err
		}
		// Keyed by track: in order with the track's other media.events.
		return platformkafka.Message{
			Topic: events.TopicMediaEvents, Key: []byte(a.TrackID.String()), Value: value,
			Headers: map[string]string{"event-type": mediav1.TypeAudioFeaturesExtracted, "event-version": fmt.Sprint(mediav1.Version), "content-type": "application/json"},
		}, nil
	}
}

// Payload maps stored features to the event; BPM 0 (no tempo) is null.
func Payload(a domain.Analysis) mediav1.AudioFeaturesExtracted {
	var bpm *float64
	if a.BPM != 0 {
		v := a.BPM
		bpm = &v
	}
	return mediav1.AudioFeaturesExtracted{
		UploadID: a.UploadID.String(), TrackID: a.TrackID.String(), BPM: bpm, BPMConfidence: a.BPMConfidence,
		IntegratedLUFS: a.IntegratedLUFS, LoudnessRangeLU: a.LoudnessRangeLU, TruePeakDBTP: a.TruePeakDBTP,
		SilenceRatio: a.SilenceRatio, AnalyzedMs: a.AnalyzedMs, AnalyzerVersion: a.AnalyzerVersion,
		SourceSHA256: a.SourceSHA256, UploadedAt: a.UploadedAt, AnalyzedAt: a.AnalyzedAt,
	}
}
