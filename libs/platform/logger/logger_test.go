package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

func TestLoggerAddsServiceAndRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Service: "catalog", Output: &buf})

	ctx := requestid.With(context.Background(), "req-1")
	log.InfoContext(ctx, "hello", "k", "v")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %v: %s", err, buf.String())
	}
	if rec["service"] != "catalog" || rec["request_id"] != "req-1" || rec["msg"] != "hello" || rec["k"] != "v" {
		t.Fatalf("unexpected record: %v", rec)
	}
}

func TestParseLevel(t *testing.T) {
	if ParseLevel("DEBUG").String() != "DEBUG" || ParseLevel("nope").String() != "INFO" {
		t.Fatal("unexpected level parsing")
	}
}
