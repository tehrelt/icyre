package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/socialv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/social/internal/domain"
)

type capture struct{ msgs []platformkafka.Message }

func (c *capture) Publish(_ context.Context, msgs ...platformkafka.Message) error {
	c.msgs = append(c.msgs, msgs...)
	return nil
}

func TestPublisher(t *testing.T) {
	c := &capture{}
	p := NewPublisher(c, "social-service")
	f := domain.Follow{FollowerID: uuid.New(), Target: domain.Target{Type: domain.TargetArtist, ID: uuid.New()}, FollowedAt: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)}
	if err := p.Followed(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if err := p.Unfollowed(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if len(c.msgs) != 2 {
		t.Fatalf("messages: %d", len(c.msgs))
	}
	m := c.msgs[0]
	if m.Topic != events.TopicSocialEvents || string(m.Key) != f.FollowerID.String() || m.Headers["event-type"] != socialv1.TypeFollowed {
		t.Fatalf("message: %+v", m)
	}
	var env struct {
		EventType string            `json:"eventType"`
		Payload   socialv1.Followed `json:"payload"`
	}
	if err := json.Unmarshal(m.Value, &env); err != nil {
		t.Fatal(err)
	}
	if env.Payload.TargetType != "artist" || env.Payload.TargetID != f.Target.ID.String() || !env.Payload.FollowedAt.Equal(f.FollowedAt) {
		t.Fatalf("payload: %+v", env)
	}
	if c.msgs[1].Headers["event-type"] != socialv1.TypeUnfollowed {
		t.Fatalf("unfollowed: %+v", c.msgs[1])
	}
}
