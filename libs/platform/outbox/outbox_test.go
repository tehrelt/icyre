package outbox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/tehrelt/icyre/libs/platform/kafka"
)

type execs struct {
	sql  []string
	args [][]any
}

func (e *execs) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.sql, e.args = append(e.sql, sql), append(e.args, args)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestWriteKeepsTraceContext(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3}, SpanID: trace.SpanID{4, 5}, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	e := &execs{}
	msg := kafka.Message{Topic: "catalog.events", Key: []byte("k"), Value: []byte(`{}`), Headers: map[string]string{"event-type": "track.created"}}
	if err := Write(ctx, e, "catalog.outbox", msg, msg); err != nil {
		t.Fatal(err)
	}
	if len(e.sql) != 2 || !strings.HasPrefix(e.sql[0], "INSERT INTO catalog.outbox ") {
		t.Fatalf("sql %v", e.sql)
	}
	var headers map[string]string
	if err := json.Unmarshal(e.args[0][3].([]byte), &headers); err != nil {
		t.Fatal(err)
	}
	if headers["event-type"] != "track.created" || !strings.Contains(headers["traceparent"], sc.TraceID().String()) {
		t.Fatalf("headers %v", headers)
	}
	if msg.Headers["traceparent"] != "" {
		t.Fatal("caller's headers must not be mutated")
	}
}

func TestTableNameIsChecked(t *testing.T) {
	for _, bad := range []string{"outbox", "catalog.outbox; DROP TABLE x", "Catalog.Outbox", ""} {
		if err := Write(context.Background(), &execs{}, bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
