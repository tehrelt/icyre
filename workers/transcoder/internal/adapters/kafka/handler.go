// Package kafka consumes track.uploaded and publishes the transcoder's
// outcome to media.events.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
)

// Transcoder is the use case the consumer drives.
type Transcoder interface {
	Handle(ctx context.Context, job mediav1.TrackUploaded) (application.Outcome, error)
}

// Metrics are the transcoder's own metrics (specs/observability).
type Metrics struct {
	duration *prometheus.HistogramVec
}

// NewMetrics registers media_transcode_duration_seconds.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "media_transcode_duration_seconds",
		Help:    "Time to process one track.uploaded by result (transcoded, duplicate, stale, failed, error).",
		Buckets: []float64{0.5, 1, 2.5, 5, 10, 20, 40, 80, 160, 320},
	}, []string{"result"})}
	reg.MustRegister(m.duration)
	return m
}

// Handler returns the media.events handler. Only track.uploaded matters;
// the topic also carries this worker's own events and ingest failures.
// Every job runs under timeout.
func Handler(x Transcoder, m *Metrics, timeout time.Duration) platformkafka.Handler {
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
