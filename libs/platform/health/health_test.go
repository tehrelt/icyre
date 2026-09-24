package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serve(c *Checker, path string) int {
	mux := http.NewServeMux()
	c.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code
}

func TestLiveIgnoresDependencies(t *testing.T) {
	c := New(time.Second)
	c.Add("db", func(context.Context) error { return errors.New("down") })
	if code := serve(c, "/health/live"); code != http.StatusOK {
		t.Fatalf("live = %d", code)
	}
}

func TestReadyReflectsChecks(t *testing.T) {
	c := New(time.Second)
	c.Add("db", func(context.Context) error { return nil })
	if code := serve(c, "/health/ready"); code != http.StatusOK {
		t.Fatalf("ready = %d", code)
	}

	c.Add("kafka", func(context.Context) error { return errors.New("down") })
	if code := serve(c, "/health/ready"); code != http.StatusServiceUnavailable {
		t.Fatalf("ready with failing check = %d", code)
	}
}

func TestDrainFailsReadiness(t *testing.T) {
	c := New(time.Second)
	c.Drain()
	if code := serve(c, "/health/ready"); code != http.StatusServiceUnavailable {
		t.Fatalf("ready while draining = %d", code)
	}
}
