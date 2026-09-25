package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
	platformkafka "github.com/tehrelt/icyre/libs/platform/kafka"
	"github.com/tehrelt/icyre/services/history/internal/domain"
)

type rec struct{ got []domain.Listen }

func (r *rec) PlayEnded(_ context.Context, l domain.Listen) (bool, error) {
	r.got = append(r.got, l)
	return true, nil
}

func record(t *testing.T, typ string, p playbackv1.Playback) platformkafka.Record {
	env, err := events.New(typ, playbackv1.Version, "playback-service", "", p.At, p)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	return platformkafka.Record{Value: raw}
}

func TestHandler(t *testing.T) {
	r := &rec{}
	h := Handler(r, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	p := playbackv1.Playback{PlaybackID: uuid.NewString(), UserID: uuid.NewString(), TrackID: uuid.NewString(), Source: "album:a", DurationMs: 1000, ListenedMs: 900, At: time.Now().UTC()}

	for _, typ := range []string{playbackv1.TypeStarted, playbackv1.TypeFinished, playbackv1.TypeSkipped} {
		if err := h(ctx, record(t, typ, p)); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.got) != 2 || r.got[0].Source != "album:a" || r.got[0].ListenedMs != 900 {
		t.Fatalf("got %+v", r.got)
	}
	bad := p
	bad.UserID = "nope"
	if err := h(ctx, record(t, playbackv1.TypeFinished, bad)); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("malformed: %v", err)
	}
	if err := h(ctx, platformkafka.Record{Value: []byte("x")}); !errors.Is(err, platformkafka.ErrPermanent) {
		t.Fatalf("garbage: %v", err)
	}
}
