package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
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
