//go:build integration

package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT is not set")
	}
	s, err := New(Config{Endpoint: endpoint, AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY")})
	if err != nil {
		t.Fatal(err)
	}
	bucket := fmt.Sprintf("it-%d", time.Now().UnixNano())
	if err := s.EnsureBucket(context.Background(), bucket); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		for obj := range s.cl.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			_ = s.cl.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{})
		}
		_ = s.cl.RemoveBucket(ctx, bucket)
	})
	return s, bucket
}

func send(t *testing.T, u SignedURL, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(u.Method, u.URL, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func TestPresignedUploadDownloadAndChecksum(t *testing.T) {
	s, bucket := newStore(t)
	ctx := context.Background()
	if err := s.Check(bucket)(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Check("missing-bucket")(ctx); err == nil {
		t.Fatal("check passed for a missing bucket")
	}

	body := []byte(strings.Repeat("icyre audio ", 1000))
	sum := sha256.Sum256(body)
	up, err := s.PresignUpload(ctx, bucket, "tracks/t1/original/source.flac", time.Minute, UploadOptions{ContentType: "audio/flac", SHA256: sum[:]})
	if err != nil {
		t.Fatal(err)
	}

	// A body that does not match the declared digest is rejected by the store.
	tampered := append([]byte("x"), body[1:]...)
	if res := send(t, up, tampered, up.Headers); res.StatusCode < 400 {
		t.Fatalf("tampered upload accepted: %d", res.StatusCode)
	}
	// Dropping the checksum header breaks the signature.
	if res := send(t, up, body, map[string]string{"Content-Type": "audio/flac"}); res.StatusCode < 400 {
		t.Fatalf("unsigned header set accepted: %d", res.StatusCode)
	}
	if res := send(t, up, body, up.Headers); res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("upload: %d %s", res.StatusCode, b)
	}
	if err := s.VerifySHA256(ctx, bucket, "tracks/t1/original/source.flac", sum[:]); err != nil {
		t.Fatal(err)
	}
	other := sha256.Sum256([]byte("other"))
	if err := s.VerifySHA256(ctx, bucket, "tracks/t1/original/source.flac", other[:]); err == nil {
		t.Fatal("checksum mismatch not detected")
	}

	// Download with Range, as the player seeks.
	down, err := s.PresignDownload(ctx, bucket, "tracks/t1/original/source.flac", time.Minute, DownloadOptions{CacheControl: "private, max-age=300"})
	if err != nil {
		t.Fatal(err)
	}
	res := send(t, down, nil, map[string]string{"Range": "bytes=0-4"})
	got, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusPartialContent || string(got) != "icyre" || res.Header.Get("Cache-Control") != "private, max-age=300" {
		t.Fatalf("range: %d %q %q", res.StatusCode, got, res.Header.Get("Cache-Control"))
	}

	// Expired URLs stop working.
	short, _ := s.PresignDownload(ctx, bucket, "tracks/t1/original/source.flac", time.Second, DownloadOptions{})
	time.Sleep(2 * time.Second)
	if res := send(t, short, nil, nil); res.StatusCode != http.StatusForbidden {
		t.Fatalf("expired URL: %d", res.StatusCode)
	}

	// Service-side put + stat.
	info, err := s.Put(ctx, bucket, "tracks/t1/audio/128.aac", bytes.NewReader(body), int64(len(body)),
		PutOptions{ContentType: "audio/aac", Metadata: map[string]string{"source-sha256": "abc"}})
	if err != nil || info.SHA256 == "" {
		t.Fatalf("put %+v %v", info, err)
	}
	st, err := s.Stat(ctx, bucket, "tracks/t1/audio/128.aac")
	if err != nil || st.Size != int64(len(body)) || st.ContentType != "audio/aac" || st.SHA256 != info.SHA256 ||
		st.Metadata["Source-Sha256"] != "abc" {
		t.Fatalf("stat %+v %v", st, err)
	}
	if _, err := s.Stat(ctx, bucket, "tracks/nope/audio/64.aac"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}

	// Streaming download.
	var buf bytes.Buffer
	if n, err := s.Download(ctx, bucket, "tracks/t1/audio/128.aac", &buf); err != nil || n != int64(len(body)) || !bytes.Equal(buf.Bytes(), body) {
		t.Fatalf("download %d %v", n, err)
	}
	if _, err := s.Download(ctx, bucket, "tracks/nope/audio/64.aac", io.Discard); !errors.Is(err, ErrNotFound) {
		t.Fatalf("download of missing: %v", err)
	}

	// Content sniffing reads only the head; removal is idempotent.
	head, err := s.ReadHead(ctx, bucket, "tracks/t1/audio/128.aac", 5)
	if err != nil || string(head) != "icyre" {
		t.Fatalf("head %q %v", head, err)
	}
	if _, err := s.ReadHead(ctx, bucket, "tracks/nope/audio/64.aac", 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("head of missing: %v", err)
	}
	for range 2 {
		if err := s.Remove(ctx, bucket, "tracks/t1/audio/128.aac"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Stat(ctx, bucket, "tracks/t1/audio/128.aac"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed: %v", err)
	}
}
