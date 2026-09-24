package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/catalogv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

type captureProducer struct{ msgs []platformkafka.Message }

func (c *captureProducer) Publish(_ context.Context, msgs ...platformkafka.Message) error {
	c.msgs = append(c.msgs, msgs...)
	return nil
}

var at = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

func TestTrackEventsMapToVersionedContract(t *testing.T) {
	artist := uuid.Must(uuid.NewV7())
	tr, err := domain.NewTrack(domain.NewTrackParams{
		ID: uuid.Must(uuid.NewV7()), AlbumID: uuid.Must(uuid.NewV7()), ArtistIDs: []uuid.UUID{artist},
		Title: "Glass Tides", Duration: 227_500 * time.Millisecond, TrackNumber: 2, Explicit: true, ISRC: "USRC17607839",
	}, at)
	if err != nil {
		t.Fatal(err)
	}

	cp := &captureProducer{}
	pub := NewPublisher(cp, "catalog-service")
	later := tr
	later.UpdatedAt = at.Add(time.Hour)
	if err := pub.Publish(context.Background(), domain.TrackCreated{Track: tr}, domain.TrackUpdated{Track: later}); err != nil {
		t.Fatal(err)
	}
	if len(cp.msgs) != 2 {
		t.Fatalf("messages = %d", len(cp.msgs))
	}

	for i, wantType := range []string{catalogv1.TypeTrackCreated, catalogv1.TypeTrackUpdated} {
		m := cp.msgs[i]
		if m.Topic != events.TopicCatalogEvents || string(m.Key) != tr.ID.String() || m.Headers["event-type"] != wantType {
			t.Fatalf("message %d routing: topic=%s key=%s headers=%v", i, m.Topic, m.Key, m.Headers)
		}
		env, err := events.Decode(m.Value)
		if err != nil {
			t.Fatal(err)
		}
		if env.EventType != wantType || env.EventVersion != 1 || env.Producer != "catalog-service" {
			t.Fatalf("envelope %d = %+v", i, env)
		}
		var p catalogv1.Track
		if err := env.DecodePayload(&p); err != nil {
			t.Fatal(err)
		}
		if p.TrackID != tr.ID.String() || p.DurationMs != 227_500 || p.Status != "DRAFT" || !p.Explicit || p.ArtistIDs[0] != artist.String() {
			t.Fatalf("payload %d = %+v", i, p)
		}
	}

	env, _ := events.Decode(cp.msgs[1].Value)
	if !env.OccurredAt.Equal(later.UpdatedAt) {
		t.Fatalf("track.updated occurredAt = %v, want %v", env.OccurredAt, later.UpdatedAt)
	}
}

func TestAlbumAndArtistEvents(t *testing.T) {
	artist, _ := domain.NewArtist(uuid.Must(uuid.NewV7()), "Nova Hale", at)
	album, _ := domain.NewAlbum(domain.NewAlbumParams{
		ID: uuid.Must(uuid.NewV7()), Title: "Prism Hours", Type: domain.AlbumTypeAlbum,
		ReleaseDate: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC), ArtistIDs: []uuid.UUID{artist.ID},
	}, at)

	cp := &captureProducer{}
	if err := NewPublisher(cp, "catalog-service").Publish(context.Background(), domain.ArtistCreated{Artist: artist}, domain.AlbumCreated{Album: album}); err != nil {
		t.Fatal(err)
	}

	env, _ := events.Decode(cp.msgs[0].Value)
	var a catalogv1.Artist
	if env.EventType != catalogv1.TypeArtistCreated || env.DecodePayload(&a) != nil || a.Name != "Nova Hale" {
		t.Fatalf("artist event = %+v / %+v", env, a)
	}

	env, _ = events.Decode(cp.msgs[1].Value)
	var al catalogv1.Album
	if env.EventType != catalogv1.TypeAlbumCreated || env.DecodePayload(&al) != nil || al.ReleaseDate != "2026-03-06" || al.AlbumType != "ALBUM" || len(al.GenreIDs) != 0 {
		t.Fatalf("album event = %+v / %+v", env, al)
	}
}
