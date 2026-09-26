package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
)

func record(t *testing.T, typ string) platformkafka.Record {
	t.Helper()
	env, err := events.New(typ, 1, "playback-service", "", time.Now(), playbackv1.Playback{
		PlaybackID: uuid.NewString(), UserID: uuid.NewString(), TrackID: uuid.NewString(), At: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	return platformkafka.Record{Value: raw}
}

func TestDecode(t *testing.T) {
	ctx := context.Background()
	if e, err := Decode(ctx, record(t, playbackv1.TypeStarted)); err != nil || e.Type != playbackv1.TypeStarted {
		t.Fatalf("started: %+v %v", e, err)
	}
	if _, err := Decode(ctx, record(t, "playback.paused")); !errors.Is(err, platformkafka.ErrSkip) {
		t.Fatalf("unknown type must be skipped: %v", err)
	}
	if _, err := Decode(ctx, platformkafka.Record{Value: []byte("{")}); err == nil || errors.Is(err, platformkafka.ErrSkip) {
		t.Fatalf("garbage must be dead-lettered: %v", err)
	}
}
