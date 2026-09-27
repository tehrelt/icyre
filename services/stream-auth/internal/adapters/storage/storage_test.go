package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
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

func (f fakeStore) Download(_ context.Context, _, key string, w io.Writer) (int64, error) {
	if !f.objects[key] {
		return 0, objectstore.ErrNotFound
	}
	n, err := io.Copy(w, strings.NewReader("adts:"+key))
	return n, err
}

func TestStream(t *testing.T) {
	m := New(fakeStore{objects: map[string]bool{"tracks/t1/audio/128.aac": true}}, "icyre-media")
	var b strings.Builder
	if n, err := m.Stream(context.Background(), "t1", media.Quality128, &b); err != nil || n == 0 || b.String() != "adts:tracks/t1/audio/128.aac" {
		t.Fatal(n, err, b.String())
	}
	if _, err := m.Stream(context.Background(), "t1", media.Quality256, io.Discard); !errors.Is(err, domain.ErrNoVariant) {
		t.Fatalf("missing variant: %v", err)
	}
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
