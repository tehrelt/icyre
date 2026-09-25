package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
)

type fakeStore struct {
	objects map[string]bool
	fail    bool
}

func (f fakeStore) Stat(_ context.Context, _, key string) (objectstore.ObjectInfo, error) {
	if f.fail {
		return objectstore.ObjectInfo{}, errors.New("storage down")
	}
	if !f.objects[key] {
		return objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return objectstore.ObjectInfo{Key: key}, nil
}

func (f fakeStore) PresignDownload(_ context.Context, bucket, key string, ttl time.Duration, o objectstore.DownloadOptions) (objectstore.SignedURL, error) {
	return objectstore.SignedURL{URL: "https://m/" + bucket + "/" + key + "?cc=" + o.CacheControl, ExpiresAt: time.Unix(0, 0).Add(ttl)}, nil
}

func TestAvailableAndSign(t *testing.T) {
	m := New(fakeStore{objects: map[string]bool{"tracks/t1/audio/64.aac": true, "tracks/t1/audio/256.aac": true}}, "icyre-media")
	ctx := context.Background()
	got, err := m.Available(ctx, "t1")
	if err != nil || len(got) != 2 || got[0] != media.Quality64 || got[1] != media.Quality256 {
		t.Fatal(got, err)
	}
	url, exp, err := m.Sign(ctx, "t1", media.Quality256, 5*time.Minute)
	if err != nil || url != "https://m/icyre-media/tracks/t1/audio/256.aac?cc=private, max-age=300" || !exp.Equal(time.Unix(300, 0)) {
		t.Fatal(url, exp, err)
	}
	if _, err := New(fakeStore{fail: true}, "b").Available(ctx, "t1"); err == nil {
		t.Fatal("storage failure hidden")
	}
}
