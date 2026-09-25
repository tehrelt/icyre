package redis

import "testing"

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
