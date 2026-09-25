//go:build integration

package redis

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// REDIS_ADDR=localhost:6379 go test -tags integration ./redis
func open(t *testing.T) *Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR is not set")
	}
	cl, err := Open(context.Background(), Config{Addr: addr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

type item struct {
	Name string `json:"name"`
}

func TestCacheGetOrLoad(t *testing.T) {
	cl := open(t)
	ctx := context.Background()
	m := NewCacheMetrics(prometheus.NewRegistry())
	c := NewCache[item](cl, fmt.Sprintf("it%d", time.Now().UnixNano()), time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)), m)

	loads := 0
	load := func(context.Context) (item, error) { loads++; return item{Name: "Nova Hale"}, nil }
	for i := 0; i < 3; i++ {
		v, err := c.GetOrLoad(ctx, "a1", load)
		if err != nil || v.Name != "Nova Hale" {
			t.Fatalf("v = %+v, err = %v", v, err)
		}
	}
	if loads != 1 {
		t.Fatalf("loader called %d times", loads)
	}
	if hits := testutil.ToFloat64(m.requests.WithLabelValues(c.name, "hit")); hits != 2 {
		t.Fatalf("hits = %v", hits)
	}
	if ttl := cl.TTL(ctx, c.key("a1")).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl = %v, entries must expire", ttl)
	}

	if err := c.Delete(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(ctx, "a1"); ok {
		t.Fatal("deleted entry still cached")
	}

	boom := errors.New("source down")
	if _, err := c.GetOrLoad(ctx, "a2", func(context.Context) (item, error) { return item{}, boom }); !errors.Is(err, boom) {
		t.Fatalf("loader error must propagate, got %v", err)
	}
}

func TestCacheSurvivesRedisOutage(t *testing.T) {
	cl := open(t)
	c := NewCache[item](cl, "outage", time.Minute, nil, nil)
	_ = cl.Close() // simulate an unreachable Redis

	v, err := c.GetOrLoad(context.Background(), "x", func(context.Context) (item, error) { return item{Name: "source"}, nil })
	if err != nil || v.Name != "source" {
		t.Fatalf("GetOrLoad must fall back to the source: %+v %v", v, err)
	}
}
