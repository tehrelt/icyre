package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCheckOrder(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	current := []Track{{TrackID: a, Position: 1}, {TrackID: b, Position: 2}, {TrackID: c, Position: 5}}
	if err := CheckOrder(current, []uuid.UUID{c, a, b}); err != nil {
		t.Fatal(err)
	}
	for name, order := range map[string][]uuid.UUID{
		"missing":   {c, a},
		"duplicate": {c, a, a},
		"foreign":   {c, a, uuid.New()},
		"extra":     {c, a, b, uuid.New()},
	} {
		if err := CheckOrder(current, order); !errors.Is(err, ErrOrderMismatch) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := CheckOrder(nil, nil); err != nil {
		t.Fatal("empty playlist, empty order:", err)
	}
}

func TestNormalizeTitle(t *testing.T) {
	if got, err := NormalizeTitle("  Late   night drives "); err != nil || got != "Late night drives" {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("я", MaxTitle+1)} {
		if _, err := NormalizeTitle(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
