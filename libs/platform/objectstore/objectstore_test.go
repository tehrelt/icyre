package objectstore

import (
	"context"
	"crypto/sha256"
	"net/url"
	"testing"
	"time"
)

func TestPresignUsesPublicEndpoint(t *testing.T) {
	s, err := New(Config{Endpoint: "minio:9000", PublicEndpoint: "media.icyre.test", PublicSecure: true, AccessKey: "k", SecretKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	d, err := s.PresignDownload(ctx, "media", "tracks/t1/audio/128.aac", 5*time.Minute, DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(d.URL)
	if u.Scheme != "https" || u.Host != "media.icyre.test" || u.Path != "/media/tracks/t1/audio/128.aac" || u.Query().Get("X-Amz-Expires") != "300" {
		t.Fatalf("url %s", d.URL)
	}
	if time.Until(d.ExpiresAt) > 5*time.Minute || time.Until(d.ExpiresAt) < 4*time.Minute {
		t.Fatalf("expiresAt %v", d.ExpiresAt)
	}

	sum := sha256.Sum256([]byte("x"))
	up, err := s.PresignUpload(ctx, "media", "k", time.Minute, UploadOptions{ContentType: "audio/flac", SHA256: sum[:]})
	if err != nil || up.Headers["X-Amz-Checksum-Sha256"] == "" || up.Headers["Content-Type"] != "audio/flac" {
		t.Fatalf("upload %+v %v", up, err)
	}
	if _, err := s.PresignUpload(ctx, "media", "k", time.Minute, UploadOptions{SHA256: []byte("short")}); err == nil {
		t.Fatal("bad digest length accepted")
	}
}
