package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

func TestLoggerAddsServiceAndRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Service: "catalog", Output: &buf})

	ctx := requestid.With(context.Background(), "req-1")
	log.InfoContext(ctx, "hello", "k", "v")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %v: %s", err, buf.String())
	}
	if rec["service"] != "catalog" || rec["request_id"] != "req-1" || rec["msg"] != "hello" || rec["k"] != "v" {
		t.Fatalf("unexpected record: %v", rec)
	}
}

func TestParseLevel(t *testing.T) {
	if ParseLevel("DEBUG").String() != "DEBUG" || ParseLevel("nope").String() != "INFO" {
		t.Fatal("unexpected level parsing")
	}
}

type stringerVal string

func (s stringerVal) String() string { return string(s) }

func TestLoggerRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Service: "auth", Output: &buf})

	log.Info("attempt",
		"password", "hunter2",
		"refresh_token", "rt-123",
		"Authorization", "Bearer eyJ.abc.def",
		"url", "https://minio.local/media/a.aac?X-Amz-Signature=deadbeef",
		"db", "postgres://icyre:secretpass@postgres:5432/icyre",
		"cache", "rediss://:secret@redis:6379",
		"webhook", "https://user:pass@example.com/path",
		"http_webhook", "http://user:pass@example.com/path",
		"wrapped_dsn", "connect postgres://icyre:secret@postgres:5432/icyre: connection refused",
		"wrapped_bearer", "auth failed: Bearer eyJ.abc.def",
		"stringer_secret", stringerVal("dial rediss://:secret@redis:6379: i/o timeout"),
		"public_url", "https://example.com/path",
		"public_email_url", "https://example.com:8443/path?email=a@b.com",
		"key", "users/u1/avatars/a1.png",
		"kid", "icyre-dev-1",
	)

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %v: %s", err, buf.String())
	}
	for _, k := range []string{
		"password", "refresh_token", "Authorization", "url", "db",
		"cache", "webhook", "http_webhook", "wrapped_dsn", "wrapped_bearer", "stringer_secret",
	} {
		if rec[k] != redactedValue {
			t.Errorf("%s = %v, want %s", k, rec[k], redactedValue)
		}
	}
	if rec["public_url"] != "https://example.com/path" ||
		rec["public_email_url"] != "https://example.com:8443/path?email=a@b.com" ||
		rec["key"] != "users/u1/avatars/a1.png" ||
		rec["kid"] != "icyre-dev-1" {
		t.Errorf("non-secret fields redacted: %v", rec)
	}
}

func TestLoggerRedactsErrorSecrets(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantRedact bool
	}{
		{
			name:       "signed_url",
			err:        errors.New("fetch https://minio.local/media/a.aac?X-Amz-Signature=deadbeef: 403"),
			wantRedact: true,
		},
		{
			name:       "dsn_password",
			err:        errors.New("connect postgres://icyre:secret@postgres:5432/icyre: connection refused"),
			wantRedact: true,
		},
		{
			name:       "wrapped_bearer",
			err:        errors.New("auth failed: Bearer eyJ.abc.def"),
			wantRedact: true,
		},
		{
			name:       "safe_dsn_without_password",
			err:        errors.New("connect postgres://postgres:5432/icyre: connection refused"),
			wantRedact: false,
		},
	}

	for _, format := range []string{"json", "text"} {
		for _, tc := range cases {
			t.Run(format+"/"+tc.name, func(t *testing.T) {
				var buf bytes.Buffer
				log := New(Options{Service: "auth", Format: format, Output: &buf})
				log.Error("op failed", Err(tc.err))

				out := buf.String()
				if format == "json" {
					var rec map[string]any
					if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
						t.Fatalf("not json: %v: %s", err, out)
					}
					got, _ := rec["error"].(string)
					if tc.wantRedact && got != redactedValue {
						t.Fatalf("error = %q, want %q", got, redactedValue)
					}
					if !tc.wantRedact && got != tc.err.Error() {
						t.Fatalf("error = %q, want %q", got, tc.err.Error())
					}
					return
				}

				if tc.wantRedact {
					if !strings.Contains(out, "error="+redactedValue) {
						t.Fatalf("expected redacted error in text output, got: %s", out)
					}
					if strings.Contains(out, "deadbeef") || strings.Contains(out, "secret") || strings.Contains(out, "eyJ.abc.def") {
						t.Fatalf("secret leaked in text output: %s", out)
					}
				} else if !strings.Contains(out, tc.err.Error()) {
					t.Fatalf("expected non-secret error preserved in text output, got: %s", out)
				}
			})
		}
	}
}
