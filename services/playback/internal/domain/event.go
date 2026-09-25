// Package domain validates playback telemetry reported by the player.
package domain

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Kind of playback event.
type Kind string

// Kinds.
const (
	Started  Kind = "started"
	Finished Kind = "finished"
	Skipped  Kind = "skipped"
)

// ValidationError lists invalid fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Limits: a track is at most 2 hours; clients may report a little more than
// the duration (rounding, buffering).
const (
	MaxDurationMs = 2 * 60 * 60 * 1000
	listenSlackMs = 5_000
)

var sourcePattern = regexp.MustCompile(`^(album|playlist|artist|search|queue):[A-Za-z0-9-]{1,64}$`)

// Event is one report from the player.
type Event struct {
	Kind       Kind
	PlaybackID uuid.UUID
	UserID     uuid.UUID
	TrackID    uuid.UUID
	Source     string
	DurationMs int64
	ListenedMs int64
	At         time.Time
}

// Validate checks a report. Telemetry comes from the client: bounds keep a
// buggy or hostile player from poisoning history and analytics.
func (e Event) Validate() error {
	f := map[string]string{}
	switch e.Kind {
	case Started, Finished, Skipped:
	default:
		f["type"] = "must be started, finished or skipped"
	}
	if e.PlaybackID == uuid.Nil {
		f["playbackId"] = "must be a UUID"
	}
	if e.TrackID == uuid.Nil {
		f["trackId"] = "must be a track ID"
	}
	if e.Source != "" && !sourcePattern.MatchString(e.Source) {
		f["source"] = "must look like album:<id>"
	}
	if e.DurationMs <= 0 || e.DurationMs > MaxDurationMs {
		f["durationMs"] = "must be between 1 and 7200000"
	}
	if e.ListenedMs < 0 || (e.DurationMs > 0 && e.ListenedMs > e.DurationMs+listenSlackMs) {
		f["listenedMs"] = "must be between 0 and the duration"
	}
	if len(f) > 0 {
		return &ValidationError{Fields: f}
	}
	return nil
}
