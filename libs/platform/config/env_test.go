package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestEnvReadsTypedValues(t *testing.T) {
	env := NewFromMap(map[string]string{
		"NAME":    "catalog",
		"PORT":    "8080",
		"DEBUG":   "true",
		"TIMEOUT": "3s",
		"BROKERS": "a:9092, b:9092,",
	})

	if got := env.String("NAME", "x"); got != "catalog" {
		t.Fatalf("String = %q", got)
	}
	if got := env.String("MISSING", "def"); got != "def" {
		t.Fatalf("String default = %q", got)
	}
	if got := env.Int("PORT", 0); got != 8080 {
		t.Fatalf("Int = %d", got)
	}
	if got := env.Bool("DEBUG", false); !got {
		t.Fatalf("Bool = %v", got)
	}
	if got := env.Duration("TIMEOUT", 0); got != 3*time.Second {
		t.Fatalf("Duration = %v", got)
	}
	if got := env.Strings("BROKERS", nil); len(got) != 2 || got[1] != "b:9092" {
		t.Fatalf("Strings = %v", got)
	}
	if err := env.Err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnvCollectsAllErrors(t *testing.T) {
	env := NewFromMap(map[string]string{"PORT": "eighty", "EMPTY": "  "})

	env.RequiredString("DSN")
	env.RequiredString("EMPTY")
	env.Int("PORT", 1)

	err := env.Err()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, key := range []string{"DSN", "EMPTY", "PORT"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not mention %s", err, key)
		}
	}
}

func TestSecretRedactsFormatting(t *testing.T) {
	env := NewFromMap(map[string]string{"API_SECRET": "super-secret-value"})
	sec := env.RequiredSecret("API_SECRET")
	if sec.Value() != "super-secret-value" {
		t.Fatalf("Value = %q", sec.Value())
	}
	if got := fmt.Sprintf("%s %v %#v", sec, sec, sec); strings.Contains(got, "super-secret-value") {
		t.Fatalf("secret leaked in formatting: %q", got)
	}
	if raw, err := sec.MarshalText(); err != nil || string(raw) != Redacted {
		t.Fatalf("MarshalText = %q, %v", raw, err)
	}
	if empty := env.Secret("MISSING_SECRET", ""); empty.String() != "" {
		t.Fatalf("empty secret String = %q", empty.String())
	}
}
