package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func valid() Event {
	return Event{Kind: Finished, PlaybackID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(), Source: "album:01a0d5c2-91cb", DurationMs: 240000, ListenedMs: 240000}
}

func TestValidate(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Event){
		"type":       func(e *Event) { e.Kind = "paused" },
		"playbackId": func(e *Event) { e.PlaybackID = uuid.Nil },
		"trackId":    func(e *Event) { e.TrackID = uuid.Nil },
		"source":     func(e *Event) { e.Source = "http://evil" },
		"durationMs": func(e *Event) { e.DurationMs = 0 },
		"listenedMs": func(e *Event) { e.ListenedMs = e.DurationMs + 60000 },
	}
	for field, mutate := range cases {
		e := valid()
		mutate(&e)
		var ve *ValidationError
		if err := e.Validate(); !errors.As(err, &ve) || ve.Fields[field] == "" {
			t.Errorf("%s: %v", field, err)
		}
	}
}
