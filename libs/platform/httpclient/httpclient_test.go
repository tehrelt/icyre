package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

func TestForwardsRequestIDAndTimesOut(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get(requestid.Header)
		if r.URL.Path == "/slow" {
			time.Sleep(200 * time.Millisecond)
		}
	}))
	defer srv.Close()

	cl := New(Config{Timeout: 50 * time.Millisecond})
	ctx := requestid.With(context.Background(), "req-42")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/fast", nil)
	res, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if got != "req-42" {
		t.Fatalf("X-Request-ID = %q", got)
	}

	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/slow", nil)
	if res, err := cl.Do(req); err == nil {
		_ = res.Body.Close()
		t.Fatal("expected timeout")
	}
}

func TestDoesNotFollowRedirects(t *testing.T) {
	var targetHit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/secret", http.StatusFound)
	}))
	defer redirector.Close()

	cl := New(Config{})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, redirector.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cl.Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusFound {
		t.Fatalf("StatusCode = %d, want %d", res.StatusCode, http.StatusFound)
	}
	if got := res.Header.Get("Location"); got != target.URL+"/secret" {
		t.Fatalf("Location = %q, want %q", got, target.URL+"/secret")
	}
	if targetHit.Load() {
		t.Fatal("redirect target was hit; client must not follow redirects")
	}
}
