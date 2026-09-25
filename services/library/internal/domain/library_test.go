package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	c := Cursor{SavedAt: time.Date(2026, 9, 25, 10, 0, 0, 123456000, time.UTC), EntityID: uuid.New()}
	got, err := DecodeCursor(c.Encode())
	if err != nil || !got.SavedAt.Equal(c.SavedAt) || got.EntityID != c.EntityID {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"", "!!", "e30"} { // e30 = "{}"
		if _, err := DecodeCursor(bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
