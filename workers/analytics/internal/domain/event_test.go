package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
)

func envelope(t *testing.T, typ string, version int, p playbackv1.Playback) events.Envelope {
	t.Helper()
	env, err := events.New(typ, version, "playback-service", "", time.Now(), p)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func validPlayback() playbackv1.Playback {
	return playbackv1.Playback{
		PlaybackID: uuid.NewString(), UserID: uuid.NewString(), TrackID: uuid.NewString(), Source: "album:x",
		DurationMs: 200_000, ListenedMs: 190_000, At: time.Date(2026, 9, 27, 21, 0, 0, 0, time.FixedZone("MSK", 3*3600)),
	}
}

func TestFromEnvelope(t *testing.T) {
	p := validPlayback()
	env := envelope(t, playbackv1.TypeFinished, playbackv1.Version, p)
	e, err := FromEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if e.EventID.String() != env.EventID || e.TrackID.String() != p.TrackID || e.Type != playbackv1.TypeFinished ||
		e.ListenedMs != 190_000 || e.At.Location() != time.UTC || e.At.Hour() != 18 {
		t.Fatalf("event %+v", e)
	}
}

func TestFromEnvelopeRejects(t *testing.T) {
	bad := validPlayback()
	bad.TrackID = "not-a-uuid"
	negative := validPlayback()
	negative.ListenedMs = -1
	noTime := validPlayback()
	noTime.At = time.Time{}

	if _, err := FromEnvelope(envelope(t, "playback.paused", 1, validPlayback())); !errors.Is(err, ErrUnknownType) {
		t.Errorf("unknown type: %v", err)
	}
	for name, env := range map[string]events.Envelope{
		"version":  envelope(t, playbackv1.TypeStarted, 2, validPlayback()),
		"bad id":   envelope(t, playbackv1.TypeStarted, 1, bad),
		"negative": envelope(t, playbackv1.TypeSkipped, 1, negative),
		"no time":  envelope(t, playbackv1.TypeSkipped, 1, noTime),
	} {
		_, err := FromEnvelope(env)
		if err == nil || errors.Is(err, ErrUnknownType) {
			t.Errorf("%s: want a permanent error, got %v", name, err)
		}
	}
}

func TestCompletionRate(t *testing.T) {
	if r := (Stats{}).CompletionRate(); r != 0 {
		t.Errorf("no ended plays: %v", r)
	}
	if r := (Stats{Plays: 10, Completions: 3, Skips: 1}).CompletionRate(); r != 0.75 {
		t.Errorf("rate %v", r)
	}
}
