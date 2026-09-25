// Package playbackv1 holds version 1 payloads of Playback Service events,
// published to events.TopicPlaybackEvents keyed by user ID (per-user order).
package playbackv1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types (specs/services/playback.md).
const (
	TypeStarted  = "playback.started"
	TypeFinished = "playback.finished"
	TypeSkipped  = "playback.skipped"
)

// Playback is the payload of every playback event. PlaybackID identifies one
// play of one track (client-generated): started, then finished or skipped.
type Playback struct {
	PlaybackID string    `json:"playbackId"`
	UserID     string    `json:"userId"`
	TrackID    string    `json:"trackId"`
	// Source is what the track was played from, e.g. "album:<id>" (optional).
	Source string `json:"source,omitempty"`
	// DurationMs is the track length; ListenedMs the time actually played
	// (seeking does not count), both as reported by the client.
	DurationMs int64     `json:"durationMs"`
	ListenedMs int64     `json:"listenedMs"`
	At         time.Time `json:"at"`
}
