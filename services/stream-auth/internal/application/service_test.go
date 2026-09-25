package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

type fakeTracks map[string]string

func (f fakeTracks) Status(_ context.Context, id string) (string, error) {
	s, ok := f[id]
	if !ok {
		return "", domain.ErrTrackNotFound
	}
	return s, nil
}

type fakeVariants map[string][]media.Quality

func (f fakeVariants) Available(_ context.Context, id string) ([]media.Quality, error) {
	return f[id], nil
}

type fakeSigner struct{ calls int }

func (f *fakeSigner) Sign(_ context.Context, id string, q media.Quality, ttl time.Duration) (string, time.Time, error) {
	f.calls++
	return "https://media.test/" + media.TrackAudioKey(id, q), time.Unix(0, 0).Add(ttl), nil
}

type auditLog []Decision

func (a *auditLog) Record(_ context.Context, d Decision) { *a = append(*a, d) }

func TestAuthorize(t *testing.T) {
	tracks := fakeTracks{"ready": domain.StatusReady, "blocked": domain.StatusBlocked, "processing": domain.StatusProcessing, "nomedia": domain.StatusReady}
	variants := fakeVariants{"ready": {64, 128}}
	signer, audit := &fakeSigner{}, &auditLog{}
	svc := New(tracks, variants, signer, audit, 5*time.Minute)
	ctx := context.Background()

	g, err := svc.Authorize(ctx, Request{UserID: "u1", TrackID: "ready"})
	if err != nil || g.Quality != 128 || g.URL != "https://media.test/tracks/ready/audio/128.aac" || !g.ExpiresAt.Equal(time.Unix(300, 0)) {
		t.Fatalf("grant %+v %v", g, err)
	}
	if g, _ := svc.Authorize(ctx, Request{UserID: "u1", TrackID: "ready", Quality: 64}); g.Quality != 64 {
		t.Fatalf("explicit quality: %d", g.Quality)
	}

	for id, want := range map[string]error{"missing": domain.ErrTrackNotFound, "blocked": domain.ErrTrackBlocked, "processing": domain.ErrTrackNotReady, "nomedia": domain.ErrNoVariant} {
		if _, err := svc.Authorize(ctx, Request{UserID: "u1", TrackID: id}); !errors.Is(err, want) || !Denied(err) {
			t.Errorf("%s: %v", id, err)
		}
	}
	if signer.calls != 2 {
		t.Fatalf("denied requests must not sign URLs: %d", signer.calls)
	}
	if len(*audit) != 6 || (*audit)[0].Granted != 128 || (*audit)[0].Requested != domain.DefaultQuality || (*audit)[2].Err == nil {
		t.Fatalf("audit %+v", *audit)
	}
}
