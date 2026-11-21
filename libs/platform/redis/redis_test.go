package redis

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestKeyConvention(t *testing.T) {
	if got := Key("ratelimit", "login", "user@example.com", "2026092510"); got != "ratelimit:login:user@example.com:2026092510" {
		t.Fatalf("Key = %q", got)
	}
}

func TestCacheRequiresTTL(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for a cache without TTL")
		}
	}()
	NewCache[int](nil, "x", 0, nil, nil)
}

func TestConfigLogValueRedactsPassword(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	cfg := Config{
		Addr:     "redis:6379",
		Username: "default",
		Password: "redis-secret-password",
	}
	log.Info("redis config", "cfg", cfg)
	out := buf.String()
	if strings.Contains(out, "redis-secret-password") {
		t.Fatalf("password leaked in log output: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in log output: %s", out)
	}
}
