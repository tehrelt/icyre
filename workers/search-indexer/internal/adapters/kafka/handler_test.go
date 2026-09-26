package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/catalogv1"
	"github.com/tehrelt/icyre/libs/contracts/events/playlistv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

type fake struct {
	calls []string
	err   error
}

func (f *fake) TrackChanged(_ context.Context, t catalogv1.Track, at time.Time) error {
	f.calls = append(f.calls, "track:"+t.TrackID+"@"+at.Format(time.RFC3339))
	return f.err
}
func (f *fake) AlbumCreated(_ context.Context, a catalogv1.Album, _ time.Time) error {
	f.calls = append(f.calls, "album:"+a.AlbumID)
	return f.err
}
func (f *fake) ArtistCreated(_ context.Context, a catalogv1.Artist, _ time.Time) error {
	f.calls = append(f.calls, "artist:"+a.ArtistID)
	return f.err
}

func (f *fake) PlaylistChanged(_ context.Context, id string, _ time.Time) error {
	f.calls = append(f.calls, "playlist:"+id)
	return f.err
}
func (f *fake) PlaylistDeleted(_ context.Context, id string, _ time.Time) error {
	f.calls = append(f.calls, "playlist-deleted:"+id)
	return f.err
}

func TestPlaylistEvents(t *testing.T) {
	f := &fake{}
	h := Handler(f)
	ctx := context.Background()
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for _, r := range []platformkafka.Record{
		rec(t, playlistv1.TypeCreated, playlistv1.Created{PlaylistID: "p1"}, at),
		rec(t, playlistv1.TypeTrackAdded, playlistv1.TrackAdded{PlaylistID: "p1"}, at),
		rec(t, playlistv1.TypeDeleted, playlistv1.Deleted{PlaylistID: "p1"}, at),
	} {
		if err := h(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(f.calls, ",") != "playlist:p1,playlist:p1,playlist-deleted:p1" {
		t.Fatalf("calls %v", f.calls)
	}
	if err := h(ctx, rec(t, playlistv1.TypeUpdated, map[string]string{}, at)); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("missing playlistId: %v", err)
	}
}

func rec(t *testing.T, typ string, payload any, at time.Time) platformkafka.Record {
	env, err := events.New(typ, 1, "catalog-service", "", at, payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	return platformkafka.Record{Value: raw}
}

func TestHandler(t *testing.T) {
	f := &fake{}
	h := Handler(f)
	ctx := context.Background()
	at := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	for _, r := range []platformkafka.Record{
		rec(t, catalogv1.TypeTrackUpdated, catalogv1.Track{TrackID: "t1"}, at),
		rec(t, catalogv1.TypeAlbumCreated, catalogv1.Album{AlbumID: "al1"}, at),
		rec(t, catalogv1.TypeArtistCreated, catalogv1.Artist{ArtistID: "ar1"}, at),
		rec(t, "genre.created", map[string]string{}, at),
	} {
		if err := h(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.calls) != 3 || f.calls[0] != "track:t1@2026-09-25T10:00:00Z" {
		t.Fatalf("calls %v", f.calls)
	}
	if err := h(ctx, platformkafka.Record{Value: []byte("{")}); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatal(err)
	}
	f.err = application.ErrNotFound
	if err := h(ctx, rec(t, catalogv1.TypeTrackCreated, catalogv1.Track{TrackID: "t2"}, at)); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("not found must be permanent: %v", err)
	}
	f.err = errors.New("catalog down")
	if err := h(ctx, rec(t, catalogv1.TypeTrackCreated, catalogv1.Track{TrackID: "t2"}, at)); err == nil || errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("transient must be retried: %v", err)
	}
}
