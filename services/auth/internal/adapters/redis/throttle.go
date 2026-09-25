// Package redis implements Auth's Redis-backed login throttle.
package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	platformredis "github.com/tehrelt/icyre/libs/platform/redis"
)

// LoginThrottle counts failed logins per account in fixed windows:
// ratelimit:login:<sha256(email)[:16]>:<window>. Emails are hashed so no
// personal data sits in key names.
type LoginThrottle struct {
	cl     *platformredis.Client
	max    int64
	window time.Duration
	now    func() time.Time
}

// NewLoginThrottle allows max failures per window.
func NewLoginThrottle(cl *platformredis.Client, max int, window time.Duration) *LoginThrottle {
	return &LoginThrottle{cl: cl, max: int64(max), window: window, now: time.Now}
}

func (t *LoginThrottle) key(email string) (string, time.Duration) {
	sum := sha256.Sum256([]byte(email))
	now := t.now()
	bucket := now.Unix() / int64(t.window.Seconds())
	resetIn := time.Unix((bucket+1)*int64(t.window.Seconds()), 0).Sub(now)
	return platformredis.Key("ratelimit", "login", hex.EncodeToString(sum[:8]), strconv.FormatInt(bucket, 10)), resetIn
}

// Allow implements ports.LoginThrottle.
func (t *LoginThrottle) Allow(ctx context.Context, email string) (bool, time.Duration, error) {
	key, resetIn := t.key(email)
	n, err := t.cl.Get(ctx, key).Int64()
	if err != nil && err != platformredis.Nil {
		return true, 0, err
	}
	return n < t.max, resetIn, nil
}

// Failed implements ports.LoginThrottle.
func (t *LoginThrottle) Failed(ctx context.Context, email string) error {
	key, _ := t.key(email)
	pipe := t.cl.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, t.window)
	_, err := pipe.Exec(ctx)
	return err
}

// Reset implements ports.LoginThrottle.
func (t *LoginThrottle) Reset(ctx context.Context, email string) error {
	key, _ := t.key(email)
	return t.cl.Del(ctx, key).Err()
}
