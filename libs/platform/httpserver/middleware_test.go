package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestStandardStackSetsRequestIDAndMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(reg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		if requestid.From(r.Context()) == "" {
			t.Error("request id missing from context")
		}
		WriteJSON(w, http.StatusOK, map[string]string{"id": r.PathValue("id")})
	})
	h := Standard(mux, StandardOptions{Service: "test", Log: discardLogger(), Metrics: metrics})

	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	req.Header.Set(requestid.Header, "abc-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get(requestid.Header); got != "abc-123" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	// The route label is the pattern, never the concrete ID.
	if n := testutil.ToFloat64(metrics.requests.WithLabelValues("GET", "GET /items/{id}", "200")); n != 1 {
		t.Fatalf("http_requests_total = %v", n)
	}
}

func TestRequestIDRejectsGarbage(t *testing.T) {
	h := RequestID()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestid.Header, "bad id\nwith newline")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get(requestid.Header); got == "" || strings.Contains(got, " ") {
		t.Fatalf("expected generated id, got %q", got)
	}
}

func TestRecoverWritesErrorEnvelope(t *testing.T) {
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
		RequestID(), Recover(discardLogger()))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != CodeInternal || body.Error.RequestID == "" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x","extra":1}`))
	if err := DecodeJSON(httptest.NewRecorder(), req, &dst); err == nil {
		t.Fatal("expected error for unknown field")
	}
}
