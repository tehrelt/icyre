// Package redis stores transient playback sessions, queues, and progress
// throttle markers in Redis (specs/services/playback.md, specs/data/redis.md).
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

// Store persists transient playback state in Redis.
type Store struct {
	cl  *platformredis.Client
	ttl time.Duration
}

// NewStore returns a Store with the given session/queue TTL.
func NewStore(cl *platformredis.Client, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{cl: cl, ttl: ttl}
}

func sessionKey(userID uuid.UUID) string {
	return platformredis.Key("playback", userID.String())
}

func queueKey(userID uuid.UUID) string {
	return platformredis.Key("queue", userID.String())
}

func progressKey(userID, playbackID uuid.UUID) string {
	return platformredis.Key("playback", "progress", userID.String(), playbackID.String())
}

// GetState loads the user's session and queue from Redis.
func (s *Store) GetState(ctx context.Context, userID uuid.UUID) (domain.Session, domain.Queue, error) {
	vals, err := s.cl.MGet(ctx, sessionKey(userID), queueKey(userID)).Result()
	if err != nil {
		return domain.Session{}, domain.Queue{}, fmt.Errorf("redis mget playback state: %w", err)
	}
	if len(vals) < 2 || vals[0] == nil {
		return domain.Session{}, domain.Queue{}, domain.ErrSessionNotFound
	}
	sessRaw, ok := vals[0].(string)
	if !ok {
		return domain.Session{}, domain.Queue{}, errors.New("unexpected session value type in redis")
	}
	var sess domain.Session
	if err := json.Unmarshal([]byte(sessRaw), &sess); err != nil {
		return domain.Session{}, domain.Queue{}, fmt.Errorf("decode playback session: %w", err)
	}
	q := domain.Queue{UserID: userID, Tracks: []uuid.UUID{}, Order: []int{}, Cursor: -1}
	if vals[1] != nil {
		qRaw, ok := vals[1].(string)
		if !ok {
			return domain.Session{}, domain.Queue{}, errors.New("unexpected queue value type in redis")
		}
		if err := json.Unmarshal([]byte(qRaw), &q); err != nil {
			return domain.Session{}, domain.Queue{}, fmt.Errorf("decode playback queue: %w", err)
		}
	}
	return sess, q, nil
}

// SaveState writes the user's session and queue to Redis with the configured TTL.
func (s *Store) SaveState(ctx context.Context, sess domain.Session, q domain.Queue) error {
	sessBytes, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("encode playback session: %w", err)
	}
	qBytes, err := json.Marshal(q)
	if err != nil {
		return fmt.Errorf("encode playback queue: %w", err)
	}
	pipe := s.cl.TxPipeline()
	pipe.Set(ctx, sessionKey(sess.UserID), sessBytes, s.ttl)
	pipe.Set(ctx, queueKey(sess.UserID), qBytes, s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis save playback state: %w", err)
	}
	return nil
}

// AllowProgress reports whether a progress event for this playback may be
// published now, setting a short-lived marker in Redis to throttle bursts.
func (s *Store) AllowProgress(ctx context.Context, userID, playbackID uuid.UUID, interval time.Duration) (bool, error) {
	if interval <= 0 {
		return true, nil
	}
	ok, err := s.cl.SetNX(ctx, progressKey(userID, playbackID), "1", interval).Result()
	if err != nil {
		return false, fmt.Errorf("redis progress throttle: %w", err)
	}
	return ok, nil
}
