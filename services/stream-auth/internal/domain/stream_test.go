package domain

import (
	"errors"
	"testing"

	"github.com/tehrelt/icyre/libs/contracts/media"
)

func TestCheckPlayable(t *testing.T) {
	cases := map[string]error{
		StatusReady: nil, StatusBlocked: ErrTrackBlocked, StatusDeleted: ErrTrackNotFound,
		StatusDraft: ErrTrackNotReady, StatusProcessing: ErrTrackNotReady, "???": ErrTrackNotReady,
	}
	for status, want := range cases {
		if got := CheckPlayable(status); !errors.Is(got, want) {
			t.Errorf("%s: %v, want %v", status, got, want)
		}
	}
}

func TestChooseVariant(t *testing.T) {
	all := []media.Quality{256, 64, 128}
	cases := []struct {
		req   media.Quality
		avail []media.Quality
		want  media.Quality
	}{
		{256, all, 256},
		{128, all, 128},
		{256, []media.Quality{64, 128}, 128}, // downgrade to the best lower
		{64, []media.Quality{128, 256}, 128}, // nothing lower: lowest higher
	}
	for _, c := range cases {
		if got, err := ChooseVariant(c.req, c.avail); err != nil || got != c.want {
			t.Errorf("ChooseVariant(%d, %v) = %d, %v; want %d", c.req, c.avail, got, err, c.want)
		}
	}
	if _, err := ChooseVariant(128, nil); !errors.Is(err, ErrNoVariant) {
		t.Fatalf("empty: %v", err)
	}
}
