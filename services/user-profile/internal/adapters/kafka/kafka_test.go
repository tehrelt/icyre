package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/authv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
)

type registrations struct {
	calls []string
}

func (r *registrations) CreateForNewUser(_ context.Context, id uuid.UUID, email string) error {
	r.calls = append(r.calls, id.String()+" "+email)
	return nil
}

func record(t *testing.T, typ string, payload any) platformkafka.Record {
	env, err := events.New(typ, 1, "auth-service", "", time.Now(), payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	return platformkafka.Record{Topic: events.TopicAuthEvents, Value: raw}
}

func TestAuthEventsHandler(t *testing.T) {
	app := &registrations{}
	h := AuthEventsHandler(app)
	id := uuid.New()

	if err := h(context.Background(), record(t, authv1.TypeUserRegistered, authv1.UserRegistered{UserID: id.String(), Email: "rin@example.com"})); err != nil {
		t.Fatal(err)
	}
	if len(app.calls) != 1 || app.calls[0] != id.String()+" rin@example.com" {
		t.Fatalf("calls = %v", app.calls)
	}
	if err := h(context.Background(), record(t, authv1.TypeSessionCreated, authv1.SessionCreated{})); err != nil || len(app.calls) != 1 {
		t.Fatalf("other event types must be skipped: %v", err)
	}
	if err := h(context.Background(), platformkafka.Record{Value: []byte("garbage")}); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("garbage must be permanent: %v", err)
	}
	if err := h(context.Background(), record(t, authv1.TypeUserRegistered, authv1.UserRegistered{UserID: "nope"})); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("bad user id must be permanent: %v", err)
	}
}
