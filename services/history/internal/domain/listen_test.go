package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRule(t *testing.T) {
	r := DefaultRule
	cases := []struct {
		listened, duration int64
		want               bool
	}{
		{30_000, 240_000, true},  // 30 s
		{29_999, 240_000, false}, // just short, under half
		{20_000, 40_000, true},   // half of a short track
		{19_000, 40_000, false},
		{0, 10_000, false},
		{5_000, 0, false},
	}
	for _, c := range cases {
		if got := r.Counts(c.listened, c.duration); got != c.want {
			t.Errorf("Counts(%d, %d) = %v", c.listened, c.duration, got)
		}
	}
	strict := Rule{MinListen: time.Minute, MinFraction: 0.9}
	if strict.Counts(30_000, 240_000) {
		t.Fatal("thresholds are configurable")
	}
}

func TestCursor(t *testing.T) {
	c := Cursor{PlayedAt: time.Date(2026, 9, 25, 10, 0, 0, 5000, time.UTC), PlaybackID: uuid.New()}
	got, err := DecodeCursor(c.Encode())
	if err != nil || !got.PlayedAt.Equal(c.PlayedAt) || got.PlaybackID != c.PlaybackID {
		t.Fatal(got, err)
	}
	if _, err := DecodeCursor("e30"); err == nil {
		t.Fatal("empty cursor accepted")
	}
}
