package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 24, 19, 0, 0, 0, time.FixedZone("x", 3600))
	env, err := New("track.created", 1, "catalog-service", "trace", at, map[string]string{"trackId": "t1"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"eventId"`, `"eventType":"track.created"`, `"eventVersion":1`, `"occurredAt":"2026-09-24T18:00:00Z"`, `"producer"`, `"traceId"`, `"payload"`} {
		if !strings.Contains(string(data), field) {
			t.Errorf("json %s misses %s", data, field)
		}
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]string
	if err := got.DecodePayload(&p); err != nil || p["trackId"] != "t1" {
		t.Fatalf("payload = %v, err = %v", p, err)
	}
}

func TestDecodeRejectsIncompleteEnvelope(t *testing.T) {
	if _, err := Decode([]byte(`{"eventType":"x"}`)); err == nil {
		t.Fatal("expected validation error")
	}
}
