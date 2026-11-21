//go:build integration

package redis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

func TestStoreIntegration(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	ctx := context.Background()
	cl, err := platformredis.Open(ctx, platformredis.Config{Addr: addr})
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	defer func() { _ = cl.Close() }()

	store := NewStore(cl, time.Minute)
	userID := uuid.New()
	t.Cleanup(func() {
		_ = cl.Del(context.Background(), sessionKey(userID), queueKey(userID)).Err()
	})

	if _, _, err := store.GetState(ctx, userID); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	t1, t2 := uuid.New(), uuid.New()
	sess, q, _, err := domain.NewSession(userID, domain.StartSessionRequest{
		Tracks:     []uuid.UUID{t1, t2},
		StartIndex: 0,
		Source:     "album:a1",
		DurationMs: 180_000,
	}, time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState(ctx, sess, q); err != nil {
		t.Fatal(err)
	}

	gotSess, gotQ, err := store.GetState(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if gotSess.SessionID != sess.SessionID || gotSess.TrackID != t1 || len(gotQ.Tracks) != 2 {
		t.Fatalf("unexpected state: sess=%+v q=%+v", gotSess, gotQ)
	}

	playbackID := uuid.New()
	t.Cleanup(func() {
		_ = cl.Del(context.Background(), progressKey(userID, playbackID)).Err()
	})
	ok, err := store.AllowProgress(ctx, userID, playbackID, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("first AllowProgress: ok=%v err=%v", ok, err)
	}
	ok, err = store.AllowProgress(ctx, userID, playbackID, 5*time.Second)
	if err != nil || ok {
		t.Fatalf("second AllowProgress must be throttled: ok=%v err=%v", ok, err)
	}
}
