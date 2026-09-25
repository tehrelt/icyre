// Package audit records stream authorization decisions: a structured
// security log line per decision and low-cardinality metrics.
package audit

import (
	"context"
	"errors"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/services/stream-auth/internal/application"
	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

// Recorder implements application.Auditor.
type Recorder struct {
	log       *slog.Logger
	decisions *prometheus.CounterVec
}

// New registers stream_authorizations_total{result,reason}.
func New(log *slog.Logger, reg prometheus.Registerer) *Recorder {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "stream_authorizations_total",
		Help: "Stream authorization decisions by result (granted, denied, error) and reason.",
	}, []string{"result", "reason"})
	reg.MustRegister(c)
	return &Recorder{log: log.With("audit", "stream_authorization"), decisions: c}
}

// Reason is a stable label for a decision.
func Reason(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, domain.ErrTrackNotFound):
		return "track_not_found"
	case errors.Is(err, domain.ErrTrackBlocked):
		return "track_blocked"
	case errors.Is(err, domain.ErrTrackNotReady):
		return "track_not_ready"
	case errors.Is(err, domain.ErrNoVariant):
		return "no_variant"
	default:
		return "dependency_failure"
	}
}

// Record logs the decision. The signed URL is never logged: it is a bearer
// credential for the object until it expires.
func (r *Recorder) Record(ctx context.Context, d application.Decision) {
	result := "granted"
	switch {
	case d.Err == nil:
	case application.Denied(d.Err):
		result = "denied"
	default:
		result = "error"
	}
	reason := Reason(d.Err)
	r.decisions.WithLabelValues(result, reason).Inc()
	attrs := []any{
		"result", result, "reason", reason,
		"user_id", d.UserID, "session_id", d.SessionID, "track_id", d.TrackID,
		"quality_requested", int(d.Requested), "quality", int(d.Granted),
	}
	if result == "error" {
		r.log.ErrorContext(ctx, "stream authorization failed", append(attrs, "error", d.Err)...)
		return
	}
	r.log.InfoContext(ctx, "stream authorization", attrs...)
}
