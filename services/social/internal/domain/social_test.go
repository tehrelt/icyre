package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	c := Cursor{At: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC), ID: uuid.New()}
	got, err := DecodeCursor(c.Encode())
	if err != nil || !got.At.Equal(c.At) || got.ID != c.ID {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	for _, bad := range []string{"", "!!", "e30"} {
		if _, err := DecodeCursor(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestParseTargetType(t *testing.T) {
	for in, ok := range map[string]bool{"user": true, "artist": true, "album": false, "": false} {
		if _, got := ParseTargetType(in); got != ok {
			t.Errorf("%q: %v", in, got)
		}
	}
}
