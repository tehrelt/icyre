package objectstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"strings"
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

func TestValidateKey(t *testing.T) {
	valid := []string{
		"tracks/t1/audio/128.aac",
		"users/u1/avatars/a1.png",
		"k",
		"a/b/file..name.flac",
	}
	for _, key := range valid {
		if err := ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) unexpected error: %v", key, err)
		}
	}

	invalid := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"leading slash", "/leading"},
		{"trailing slash", "tracks/t1/"},
		{"dotdot middle", "a/../b"},
		{"dotdot only", ".."},
		{"dotdot start", "../a"},
		{"dotdot end", "a/.."},
		{"dot middle", "a/./b"},
		{"dot only", "."},
		{"double slash", "a//b"},
		{"backslash", `a\b`},
		{"query", "a?x=1"},
		{"fragment", "a#frag"},
		{"newline", "a\nb"},
		{"carriage return", "a\rb"},
		{"tab", "a\tb"},
		{"nul byte", "a\x00b"},
		{"del byte", "a\x7fb"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKey(tc.key)
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("ValidateKey(%q) = %v, want ErrInvalidKey", tc.key, err)
			}
		})
	}
}

func TestStoreMethodsRejectInvalidKey(t *testing.T) {
	s, err := New(Config{Endpoint: "minio:9000", AccessKey: "k", SecretKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	badKey := "tracks/../secret"

	if _, err := s.PresignDownload(ctx, "media", badKey, time.Minute, DownloadOptions{}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("PresignDownload = %v, want ErrInvalidKey", err)
	}
	if _, err := s.PresignUpload(ctx, "media", badKey, time.Minute, UploadOptions{}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("PresignUpload = %v, want ErrInvalidKey", err)
	}
	if _, err := s.Stat(ctx, "media", badKey); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Stat = %v, want ErrInvalidKey", err)
	}
	if _, err := s.Download(ctx, "media", badKey, nil); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Download = %v, want ErrInvalidKey", err)
	}
	if _, err := s.ReadHead(ctx, "media", badKey, 16); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("ReadHead = %v, want ErrInvalidKey", err)
	}
	if err := s.Remove(ctx, "media", badKey); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Remove = %v, want ErrInvalidKey", err)
	}
	if _, err := s.Put(ctx, "media", badKey, nil, 0, PutOptions{}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Put = %v, want ErrInvalidKey", err)
	}
}

func TestNewRejectsInvalidEndpoints(t *testing.T) {
	invalidEndpoints := []struct {
		name string
		ep   string
	}{
		{"empty", ""},
		{"slash", "minio:9000/bucket"},
		{"scheme", "https://minio:9000"},
		{"query", "minio:9000?x=1"},
		{"fragment", "minio:9000#frag"},
		{"userinfo", "user@minio:9000"},
		{"space", "minio 9000"},
		{"tab", "minio\t9000"},
		{"newline", "minio\n9000"},
		{"backslash", `minio\9000`},
	}

	for _, tc := range invalidEndpoints {
		t.Run("endpoint/"+tc.name, func(t *testing.T) {
			if _, err := New(Config{Endpoint: tc.ep, AccessKey: "k", SecretKey: "s"}); err == nil {
				t.Fatalf("New accepted invalid Endpoint %q", tc.ep)
			}
		})
		if tc.ep == "" {
			continue // empty PublicEndpoint defaults to Endpoint
		}
		t.Run("public_endpoint/"+tc.name, func(t *testing.T) {
			if _, err := New(Config{Endpoint: "minio:9000", PublicEndpoint: tc.ep, AccessKey: "k", SecretKey: "s"}); err == nil {
				t.Fatalf("New accepted invalid PublicEndpoint %q", tc.ep)
			}
		})
	}
}

func TestNewInvalidEndpointDoesNotLeakSecrets(t *testing.T) {
	cases := []struct {
		name   string
		ep     string
		secret string
	}{
		{"userinfo", "user:secretpass@minio:9000", "secretpass"},
		{"signed_query", "minio:9000?X-Amz-Signature=deadbeef", "deadbeef"},
	}
	for _, tc := range cases {
		t.Run("endpoint/"+tc.name, func(t *testing.T) {
			_, err := New(Config{Endpoint: tc.ep, AccessKey: "k", SecretKey: "s"})
			if err == nil {
				t.Fatalf("expected error for %q", tc.ep)
			}
			if strings.Contains(err.Error(), tc.secret) {
				t.Fatalf("error leaked secret %q: %v", tc.secret, err)
			}
		})
		t.Run("public_endpoint/"+tc.name, func(t *testing.T) {
			_, err := New(Config{Endpoint: "minio:9000", PublicEndpoint: tc.ep, AccessKey: "k", SecretKey: "s"})
			if err == nil {
				t.Fatalf("expected error for %q", tc.ep)
			}
			if strings.Contains(err.Error(), tc.secret) {
				t.Fatalf("error leaked secret %q: %v", tc.secret, err)
			}
		})
	}
}
