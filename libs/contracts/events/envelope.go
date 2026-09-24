// Package events defines the uniform Kafka event envelope and topics.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Topics (specs/data/kafka.md). The DLQ of a topic is <topic>.dlq.
const (
	TopicCatalogEvents = "catalog.events"
)

// Envelope wraps every event published to Kafka.
type Envelope struct {
	EventID      string          `json:"eventId"`
	EventType    string          `json:"eventType"`
	EventVersion int             `json:"eventVersion"`
	OccurredAt   time.Time       `json:"occurredAt"`
	Producer     string          `json:"producer"`
	TraceID      string          `json:"traceId,omitempty"`
	Payload      json.RawMessage `json:"payload"`
}

// New builds an envelope with a fresh UUIDv7 event ID.
func New(eventType string, version int, producer, traceID string, occurredAt time.Time, payload any) (Envelope, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Envelope{}, fmt.Errorf("event id: %w", err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	return Envelope{
		EventID:      id.String(),
		EventType:    eventType,
		EventVersion: version,
		OccurredAt:   occurredAt.UTC(),
		Producer:     producer,
		TraceID:      traceID,
		Payload:      raw,
	}, nil
}

// Validate checks the mandatory envelope fields.
func (e Envelope) Validate() error {
	var errs []error
	if e.EventID == "" {
		errs = append(errs, errors.New("eventId is empty"))
	}
	if e.EventType == "" {
		errs = append(errs, errors.New("eventType is empty"))
	}
	if e.EventVersion < 1 {
		errs = append(errs, errors.New("eventVersion must be >= 1"))
	}
	if e.OccurredAt.IsZero() {
		errs = append(errs, errors.New("occurredAt is empty"))
	}
	if e.Producer == "" {
		errs = append(errs, errors.New("producer is empty"))
	}
	if len(e.Payload) == 0 {
		errs = append(errs, errors.New("payload is empty"))
	}
	return errors.Join(errs...)
}

// Decode parses and validates an envelope.
func Decode(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if err := e.Validate(); err != nil {
		return Envelope{}, fmt.Errorf("invalid envelope: %w", err)
	}
	return e, nil
}

// DecodePayload unmarshals the payload into dst.
func (e Envelope) DecodePayload(dst any) error {
	return json.Unmarshal(e.Payload, dst)
}
