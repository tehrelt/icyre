package postgres

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestOperation(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM t":                      "select",
		"  insert into t values ($1)":          "insert",
		"-- name: X\nUPDATE t SET a = 1":       "update",
		"WITH x AS (SELECT 1) SELECT * FROM x": "with",
		"VACUUM":                               "other",
		"-- only a comment":                    "other",
	}
	for sql, want := range cases {
		if got := operation(sql); got != want {
			t.Errorf("operation(%q) = %q, want %q", sql, got, want)
		}
	}
}

func TestConfigLogValueRedactsDSN(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	cfg := Config{
		DSN:             "postgres://icyre:supersecret@postgres:5432/icyre",
		ApplicationName: "auth",
	}
	log.Info("db config", "cfg", cfg)
	out := buf.String()
	if strings.Contains(out, "supersecret") {
		t.Fatalf("DSN leaked in log output: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in log output: %s", out)
	}
}
