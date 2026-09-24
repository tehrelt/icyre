package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var now = time.Date(2026, 9, 24, 19, 0, 0, 0, time.UTC)

func id() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func fields(t *testing.T, err error) map[string]string {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	return ve.Fields
}

func TestNewArtist(t *testing.T) {
	a, err := NewArtist(id(), "  Nova Hale ", now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Nova Hale" || !a.CreatedAt.Equal(now) {
		t.Fatalf("unexpected artist %+v", a)
	}

	_, err = NewArtist(uuid.Nil, " ", now)
	f := fields(t, err)
	if f["id"] == "" || f["name"] == "" {
		t.Fatalf("fields = %v", f)
	}
}

func TestNewAlbumNormalisesAndValidates(t *testing.T) {
	artist := id()
	a, err := NewAlbum(NewAlbumParams{
		ID: id(), Title: "Prism Hours", Type: AlbumTypeAlbum,
		ReleaseDate: time.Date(2026, 3, 6, 15, 30, 0, 0, time.UTC),
		ArtistIDs:   []uuid.UUID{artist, artist},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.ArtistIDs) != 1 {
		t.Fatalf("artists not deduplicated: %v", a.ArtistIDs)
	}
	if a.ReleaseDate.Hour() != 0 {
		t.Fatalf("release date not truncated to a day: %v", a.ReleaseDate)
	}

	_, err = NewAlbum(NewAlbumParams{ID: id(), Type: "LP"}, now)
	f := fields(t, err)
	for _, k := range []string{"title", "albumType", "releaseDate", "artistIds"} {
		if f[k] == "" {
			t.Errorf("missing validation for %s: %v", k, f)
		}
	}
}

func validTrack(t *testing.T) Track {
	t.Helper()
	tr, err := NewTrack(NewTrackParams{
		ID: id(), AlbumID: id(), ArtistIDs: []uuid.UUID{id()},
		Title: "Glass Tides", Duration: 227 * time.Second, TrackNumber: 2, ISRC: "us-rc1-76-07839",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestNewTrackDefaults(t *testing.T) {
	tr := validTrack(t)
	if tr.Status != TrackStatusDraft || tr.DiscNumber != 1 || tr.ISRC != "USRC17607839" {
		t.Fatalf("unexpected track %+v", tr)
	}
}

func TestNewTrackValidation(t *testing.T) {
	_, err := NewTrack(NewTrackParams{ID: id(), AlbumID: id(), ArtistIDs: []uuid.UUID{id()}, Title: "x", Duration: -1, TrackNumber: 0, ISRC: "nope"}, now)
	f := fields(t, err)
	for _, k := range []string{"durationMs", "trackNumber", "isrc"} {
		if f[k] == "" {
			t.Errorf("missing validation for %s: %v", k, f)
		}
	}
}

func TestTrackStatusMachine(t *testing.T) {
	cases := []struct {
		from, to TrackStatus
		ok       bool
	}{
		{TrackStatusDraft, TrackStatusProcessing, true},
		{TrackStatusDraft, TrackStatusReady, false},
		{TrackStatusProcessing, TrackStatusReady, true},
		{TrackStatusProcessing, TrackStatusDraft, true},
		{TrackStatusReady, TrackStatusBlocked, true},
		{TrackStatusBlocked, TrackStatusReady, true},
		{TrackStatusReady, TrackStatusProcessing, false},
		{TrackStatusReady, TrackStatusDeleted, true},
		{TrackStatusDeleted, TrackStatusReady, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.ok {
			t.Errorf("%s → %s = %v, want %v", c.from, c.to, got, c.ok)
		}
	}
}

func TestTrackApply(t *testing.T) {
	tr := validTrack(t)
	later := now.Add(time.Hour)

	processing := TrackStatusProcessing
	title := " Glass Tides (Remastered) "
	changed, err := tr.Apply(TrackChanges{Status: &processing, Title: &title}, later)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if tr.Status != TrackStatusProcessing || tr.Title != "Glass Tides (Remastered)" || !tr.UpdatedAt.Equal(later) {
		t.Fatalf("unexpected track %+v", tr)
	}

	changed, err = tr.Apply(TrackChanges{Status: &processing}, later.Add(time.Hour))
	if err != nil || changed || !tr.UpdatedAt.Equal(later) {
		t.Fatalf("no-op update must not touch the track: changed=%v err=%v", changed, err)
	}

	blocked := TrackStatusBlocked
	before := tr
	if _, err := tr.Apply(TrackChanges{Status: &blocked, Title: &title}, later); !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("expected ErrInvalidStatusTransition, got %v", err)
	}
	if tr.Status != before.Status {
		t.Fatal("failed update must not mutate the track")
	}

	deleted := TrackStatusDeleted
	if _, err := tr.Apply(TrackChanges{Status: &deleted}, later); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Apply(TrackChanges{Title: &title}, later); !errors.Is(err, ErrTrackDeleted) {
		t.Fatalf("expected ErrTrackDeleted, got %v", err)
	}
}
