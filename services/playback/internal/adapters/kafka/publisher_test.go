package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

type captured struct{ msgs []platformkafka.Message }

func (c *captured) Publish(_ context.Context, m ...platformkafka.Message) error {
	c.msgs = append(c.msgs, m...)
	return nil
}

func TestPublish(t *testing.T) {
	c := &captured{}
	e := domain.Event{Kind: domain.Skipped, PlaybackID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(), Source: "album:x", DurationMs: 200000, ListenedMs: 12000, At: time.Now().UTC()}
	if err := NewPublisher(c, "playback-service").Publish(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	m := c.msgs[0]
	if m.Topic != events.TopicPlaybackEvents || string(m.Key) != e.UserID.String() || m.Headers["event-type"] != playbackv1.TypeSkipped {
		t.Fatalf("message %+v", m)
	}
	var env events.Envelope
	_ = json.Unmarshal(m.Value, &env)
	var p playbackv1.Playback
	if err := env.DecodePayload(&p); err != nil || p.ListenedMs != 12000 || p.Source != "album:x" || p.PlaybackID != e.PlaybackID.String() {
		t.Fatalf("payload %+v %v", p, err)
	}
}
