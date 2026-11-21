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
	"github.com/tehrelt/icyre/libs/contracts/events/profilev1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
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

type recProducer struct {
	msgs []platformkafka.Message
}

func (p *recProducer) Publish(_ context.Context, msgs ...platformkafka.Message) error {
	p.msgs = append(p.msgs, msgs...)
	return nil
}

func TestPublisher(t *testing.T) {
	prod := &recProducer{}
	pub := NewPublisher(prod, "user-profile-service")
	ctx := context.Background()
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()

	pr := domain.Profile{
		UserID:      userID,
		Username:    "rin",
		DisplayName: "Rin",
		AvatarKey:   "users/" + userID.String() + "/avatars/1.png",
		UpdatedAt:   now,
	}
	if err := pub.ProfileUpdated(ctx, pr); err != nil {
		t.Fatal(err)
	}
	if err := pub.AvatarChanged(ctx, userID, pr.AvatarKey, now); err != nil {
		t.Fatal(err)
	}
	if err := pub.AvatarChanged(ctx, userID, "", now); err != nil {
		t.Fatal(err)
	}
	if len(prod.msgs) != 3 {
		t.Fatalf("msgs=%d", len(prod.msgs))
	}

	for i, wantType := range []string{profilev1.TypeProfileUpdated, profilev1.TypeAvatarChanged, profilev1.TypeAvatarChanged} {
		m := prod.msgs[i]
		if m.Topic != events.TopicProfileEvents || string(m.Key) != userID.String() || m.Headers["event-type"] != wantType {
			t.Fatalf("msg[%d] = %+v", i, m)
		}
		env, err := events.Decode(m.Value)
		if err != nil || env.EventType != wantType {
			t.Fatalf("env[%d] = %+v err=%v", i, env, err)
		}
	}

	var ac profilev1.AvatarChanged
	env, _ := events.Decode(prod.msgs[1].Value)
	if err := env.DecodePayload(&ac); err != nil || ac.UserID != userID.String() || ac.AvatarKey != pr.AvatarKey {
		t.Fatalf("avatar.changed payload = %+v err=%v", ac, err)
	}

	var nop NopPublisher
	if err := nop.ProfileUpdated(ctx, pr); err != nil {
		t.Fatal(err)
	}
	if err := nop.AvatarChanged(ctx, userID, "", now); err != nil {
		t.Fatal(err)
	}
}
