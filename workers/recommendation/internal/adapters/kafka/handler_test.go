package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/libraryv1"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/workers/recommendation/internal/adapters/postgres"
)

type call struct {
	op, kind string
	user, id uuid.UUID
	at       time.Time
}

type fakeStore struct {
	calls    []call
	features []postgres.Features
}

func (f *fakeStore) SaveLike(_ context.Context, kind string, user, id uuid.UUID, at time.Time) error {
	f.calls = append(f.calls, call{"save", kind, user, id, at})
	return nil
}

func (f *fakeStore) RemoveLike(_ context.Context, kind string, user, id uuid.UUID, at time.Time) error {
	f.calls = append(f.calls, call{"remove", kind, user, id, at})
	return nil
}

func (f *fakeStore) SaveFeatures(_ context.Context, feat postgres.Features) error {
	f.features = append(f.features, feat)
	return nil
}

func rec(t *testing.T, typ string, payload any) platformkafka.Record {
	t.Helper()
	env, err := events.New(typ, 1, "test", "", time.Now(), payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	return platformkafka.Record{Value: raw}
}

func TestHandlerRoutesLikesAndFeatures(t *testing.T) {
	s := &fakeStore{}
	h := Handler(s)
	ctx := context.Background()
	user, track, album := uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	bpm := 120.0
	for _, r := range []platformkafka.Record{
		rec(t, libraryv1.TypeTrackSaved, libraryv1.TrackSaved{UserID: user.String(), TrackID: track.String(), SavedAt: at}),
		rec(t, libraryv1.TypeTrackRemoved, libraryv1.TrackRemoved{UserID: user.String(), TrackID: track.String(), RemovedAt: at}),
		rec(t, libraryv1.TypeAlbumSaved, libraryv1.AlbumSaved{UserID: user.String(), AlbumID: album.String(), SavedAt: at}),
		rec(t, libraryv1.TypeAlbumRemoved, libraryv1.AlbumRemoved{UserID: user.String(), AlbumID: album.String(), RemovedAt: at}),
		rec(t, mediav1.TypeAudioFeaturesExtracted, mediav1.AudioFeaturesExtracted{TrackID: track.String(), BPM: &bpm, AnalyzerVersion: "v1", UploadedAt: at}),
		rec(t, mediav1.TypeTrackUploaded, mediav1.TrackUploaded{TrackID: track.String()}), // ignored
	} {
		if err := h(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	want := []call{
		{"save", postgres.KindTrack, user, track, at}, {"remove", postgres.KindTrack, user, track, at},
		{"save", postgres.KindAlbum, user, album, at}, {"remove", postgres.KindAlbum, user, album, at},
	}
	if len(s.calls) != len(want) {
		t.Fatalf("calls %+v", s.calls)
	}
	for i := range want {
		if s.calls[i] != want[i] {
			t.Fatalf("call %d: %+v, want %+v", i, s.calls[i], want[i])
		}
	}
	if len(s.features) != 1 || *s.features[0].BPM != 120 || s.features[0].TrackID != track {
		t.Fatalf("features %+v", s.features)
	}
}

func TestHandlerRejectsMalformed(t *testing.T) {
	h := Handler(&fakeStore{})
	for name, r := range map[string]platformkafka.Record{
		"garbage":  {Value: []byte("{")},
		"bad id":   rec(t, libraryv1.TypeTrackSaved, libraryv1.TrackSaved{UserID: "x", TrackID: uuid.NewString(), SavedAt: time.Now()}),
		"no time":  rec(t, libraryv1.TypeTrackSaved, libraryv1.TrackSaved{UserID: uuid.NewString(), TrackID: uuid.NewString()}),
		"features": rec(t, mediav1.TypeAudioFeaturesExtracted, mediav1.AudioFeaturesExtracted{TrackID: uuid.NewString()}),
	} {
		if err := h(context.Background(), r); !errors.Is(err, platformkafka.ErrPermanent) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
