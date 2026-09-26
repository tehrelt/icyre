package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/workers/transcoder/internal/application"
)

type object struct {
	body []byte
	info objectstore.ObjectInfo
}

// memStore mimics the object store: user metadata comes back canonical.
type memStore map[string]object

func (m memStore) Stat(_ context.Context, bucket, key string) (objectstore.ObjectInfo, error) {
	o, ok := m[bucket+"/"+key]
	if !ok {
		return objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return o.info, nil
}

func (m memStore) Download(_ context.Context, bucket, key string, w io.Writer) (int64, error) {
	o, ok := m[bucket+"/"+key]
	if !ok {
		return 0, objectstore.ErrNotFound
	}
	n, err := w.Write(o.body)
	return int64(n), err
}

func (m memStore) Put(_ context.Context, bucket, key string, r io.Reader, size int64, o objectstore.PutOptions) (objectstore.ObjectInfo, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return objectstore.ObjectInfo{}, err
	}
	if int64(len(b)) != size {
		return objectstore.ObjectInfo{}, errors.New("size mismatch")
	}
	md := map[string]string{}
	for k, v := range o.Metadata {
		md[http.CanonicalHeaderKey(k)] = v
	}
	info := objectstore.ObjectInfo{Key: key, Size: size, ContentType: o.ContentType, Metadata: md}
	m[bucket+"/"+key] = object{body: b, info: info}
	return info, nil
}

func TestDownloadHashesTheMaster(t *testing.T) {
	store := memStore{"in/tracks/t1/original/u1.flac": {body: []byte("master bytes")}}
	m := New(store, "icyre-media")
	path := filepath.Join(t.TempDir(), "source")
	sum, err := m.Download(context.Background(), "in", "tracks/t1/original/u1.flac", path)
	want := sha256.Sum256([]byte("master bytes"))
	if err != nil || sum != hex.EncodeToString(want[:]) {
		t.Fatal(sum, err)
	}
	if b, _ := os.ReadFile(path); string(b) != "master bytes" {
		t.Fatalf("%q", b)
	}
	if _, err := m.Download(context.Background(), "in", "nope", path); !errors.Is(err, application.ErrSourceMissing) {
		t.Fatal(err)
	}
}

func TestVariantRoundTripsProvenance(t *testing.T) {
	store := memStore{}
	m := New(store, "icyre-media")
	ctx := context.Background()
	if _, ok, err := m.Variant(ctx, "t1", media.Quality128); ok || err != nil {
		t.Fatal(ok, err)
	}

	file := filepath.Join(t.TempDir(), "128.aac")
	if err := os.WriteFile(file, bytes.Repeat([]byte{1}, 42), 0o600); err != nil {
		t.Fatal(err)
	}
	p := application.Provenance{UploadID: "u1", SourceSHA256: "aa", UploadedAt: time.Date(2026, 9, 26, 12, 0, 0, 123, time.UTC), DurationMs: 180_000}
	put, err := m.PutVariant(ctx, "t1", media.Quality128, file, p)
	if err != nil || put.Key != "tracks/t1/audio/128.aac" || put.SizeBytes != 42 {
		t.Fatal(put, err)
	}
	if ct := store["icyre-media/tracks/t1/audio/128.aac"].info.ContentType; ct != media.AudioContentType {
		t.Fatal(ct)
	}

	got, ok, err := m.Variant(ctx, "t1", media.Quality128)
	if err != nil || !ok || got.Provenance != p || got.Quality != media.Quality128 || got.SizeBytes != 42 {
		t.Fatalf("%+v %v %v", got, ok, err)
	}
}

func TestVariantWithoutProvenance(t *testing.T) {
	store := memStore{"icyre-media/tracks/t1/audio/64.aac": {info: objectstore.ObjectInfo{Size: 7}}}
	got, ok, err := New(store, "icyre-media").Variant(context.Background(), "t1", media.Quality64)
	if err != nil || !ok || !got.UploadedAt.IsZero() || got.SourceSHA256 != "" {
		t.Fatalf("%+v %v %v", got, ok, err)
	}
}
