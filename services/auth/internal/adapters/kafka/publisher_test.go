package kafka

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/authv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

type capture struct{ msgs []platformkafka.Message }

func (c *capture) Publish(_ context.Context, m ...platformkafka.Message) error {
	c.msgs = append(c.msgs, m...)
	return nil
}

func TestUserRegisteredNeverCarriesCredentials(t *testing.T) {
	acc := domain.NewAccount(uuid.New(), "rin@example.com", "$argon2id$secret", time.Now())
	c := &capture{}
	if err := NewPublisher(c, "auth-service").Publish(context.Background(), domain.UserRegistered{Account: acc}); err != nil {
		t.Fatal(err)
	}
	m := c.msgs[0]
	if m.Topic != events.TopicAuthEvents || string(m.Key) != acc.ID.String() {
		t.Fatalf("routing %s %s", m.Topic, m.Key)
	}
	if strings.Contains(string(m.Value), "argon2") {
		t.Fatal("password hash leaked into an event")
	}
	env, err := events.Decode(m.Value)
	if err != nil {
		t.Fatal(err)
	}
	var p authv1.UserRegistered
	if err := env.DecodePayload(&p); err != nil || p.Email != "rin@example.com" || env.EventType != authv1.TypeUserRegistered {
		t.Fatalf("payload %+v env %+v err %v", p, env, err)
	}
}
