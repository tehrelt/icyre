// Package kafka provides a producer and a consumer helper built on franz-go.
//
// It works with raw bytes: event envelopes and payload schemas live in
// libs/contracts. Delivery is at-least-once, so consumers must be idempotent.
package kafka

import (
	"context"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// DLQSuffix is appended to a topic name to form its dead-letter topic.
const DLQSuffix = ".dlq"

// DLQTopic returns the dead-letter topic for topic (e.g. catalog.events.dlq).
func DLQTopic(topic string) string { return topic + DLQSuffix }

// Message is an outgoing record.
type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// Record is an incoming record.
type Record struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Timestamp time.Time
}

func headersToMap(hs []kgo.RecordHeader) map[string]string {
	m := make(map[string]string, len(hs))
	for _, h := range hs {
		m[h.Key] = string(h.Value)
	}
	return m
}

func mapToHeaders(m map[string]string) []kgo.RecordHeader {
	hs := make([]kgo.RecordHeader, 0, len(m))
	for k, v := range m {
		hs = append(hs, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	return hs
}

// injectTrace writes the W3C trace context of ctx into headers.
func injectTrace(ctx context.Context, headers map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
}

// extractTrace returns ctx enriched with the trace context found in headers.
func extractTrace(ctx context.Context, headers map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}
