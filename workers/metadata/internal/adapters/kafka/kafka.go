// Package kafka consumes track.uploaded and encodes media.metadata_extracted.
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
	"github.com/tehrelt/icyre/workers/metadata/internal/application"
	"github.com/tehrelt/icyre/workers/metadata/internal/domain"
)

// Extractor is the use case the consumer drives.
type Extractor interface {
	Handle(ctx context.Context, job mediav1.TrackUploaded) (application.Outcome, error)
}

// Metrics are the worker's own metrics (specs/observability).
type Metrics struct {
	duration *prometheus.HistogramVec
}

// NewMetrics registers media_metadata_extract_duration_seconds.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "media_metadata_extract_duration_seconds",
		Help:    "Time to process one track.uploaded by result (extracted, duplicate, missing, undecodable, error).",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"result"})}
	reg.MustRegister(m.duration)
	return m
}

// Handler returns the media.events handler. Only track.uploaded matters;
// the topic also carries this worker's own events. Every job runs under
// timeout.
func Handler(x Extractor, m *Metrics, timeout time.Duration) platformkafka.Handler {
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

// Encoder returns the media.metadata_extracted encoder; source names the
// producing service in the envelope.
func Encoder(source string) func(ctx context.Context, m domain.Metadata) (platformkafka.Message, error) {
	return func(ctx context.Context, m domain.Metadata) (platformkafka.Message, error) {
		traceID := ""
		if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
			traceID = sc.TraceID().String()
		}
		env, err := events.New(mediav1.TypeMetadataExtracted, mediav1.Version, source, traceID, m.ExtractedAt, mediav1.MetadataExtracted{
			UploadID: m.UploadID.String(), TrackID: m.TrackID.String(), Container: m.Container, Codec: m.Codec,
			DurationMs: m.DurationMs, BitrateBps: m.BitrateBps, SampleRateHz: m.SampleRateHz, Channels: m.Channels,
			SourceSHA256: m.SourceSHA256, UploadedAt: m.UploadedAt, ExtractedAt: m.ExtractedAt,
		})
		if err != nil {
			return platformkafka.Message{}, err
		}
		value, err := json.Marshal(env)
		if err != nil {
			return platformkafka.Message{}, err
		}
		// Keyed by track: in order with the track's other media.events.
		return platformkafka.Message{
			Topic: events.TopicMediaEvents, Key: []byte(m.TrackID.String()), Value: value,
			Headers: map[string]string{"event-type": mediav1.TypeMetadataExtracted, "event-version": fmt.Sprint(mediav1.Version), "content-type": "application/json"},
		}, nil
	}
}
