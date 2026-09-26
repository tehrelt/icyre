package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
)

type capture struct{ msgs []platformkafka.Message }

func (c *capture) Publish(_ context.Context, msgs ...platformkafka.Message) error {
	c.msgs = append(c.msgs, msgs...)
	return nil
}

func TestPublishesKeyedByTrack(t *testing.T) {
	c := &capture{}
	p := NewPublisher(c, "transcoder")
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if err := p.Transcoded(context.Background(), mediav1.TrackTranscoded{TrackID: "t1", DurationMs: 1000, TranscodedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := p.Failed(context.Background(), mediav1.TranscodeFailed{TrackID: "t2", Reason: mediav1.ReasonUndecodable, FailedAt: at}); err != nil {
		t.Fatal(err)
	}
	if len(c.msgs) != 2 {
		t.Fatal(c.msgs)
	}
	for i, want := range []struct{ key, typ string }{{"t1", mediav1.TypeTrackTranscoded}, {"t2", mediav1.TypeTranscodeFailed}} {
		m := c.msgs[i]
		env, err := events.Decode(m.Value)
		if err != nil || m.Topic != events.TopicMediaEvents || string(m.Key) != want.key || env.EventType != want.typ ||
			m.Headers["event-type"] != want.typ || env.Producer != "transcoder" || !env.OccurredAt.Equal(at) {
			t.Fatalf("%d: %+v %+v %v", i, m, env, err)
		}
	}
	var got mediav1.TrackTranscoded
	env, _ := events.Decode(c.msgs[0].Value)
	if err := env.DecodePayload(&got); err != nil || got.DurationMs != 1000 {
		t.Fatal(got, err)
	}
}
