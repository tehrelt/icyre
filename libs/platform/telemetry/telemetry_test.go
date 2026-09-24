package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestSetupTracingInstallsProvider(t *testing.T) {
	shutdown, err := SetupTracing(context.Background(), TracingConfig{ServiceName: "test", Version: "1", Environment: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	_, span := otel.Tracer("t").Start(context.Background(), "op")
	defer span.End()
	if !span.SpanContext().HasTraceID() {
		t.Fatal("spans must carry trace IDs even when exporting is disabled")
	}
}

func TestMetricsHandlerServesRuntimeMetrics(t *testing.T) {
	rec := httptest.NewRecorder()
	MetricsHandler(NewRegistry()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("status = %d", rec.Code)
	}
}
