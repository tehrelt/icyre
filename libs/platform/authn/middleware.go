package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/redis"
)

// CodeUnauthenticated is the error-model code for missing/invalid tokens.
const CodeUnauthenticated = "UNAUTHENTICATED"

// bearer extracts the token from "Authorization: Bearer <token>".
func bearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", ErrNoToken
	}
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", ErrInvalidToken
	}
	return strings.TrimSpace(token), nil
}

// Required rejects requests without a valid access token (401).
func (v *Verifier) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := bearer(r)
		if err != nil {
			unauthorized(w, r, err)
			return
		}
		p, err := v.Verify(r.Context(), raw)
		if err != nil {
			unauthorized(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

// Optional attaches the principal when a valid token is present and lets
// anonymous requests through; an invalid token is still rejected.
func (v *Verifier) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := bearer(r)
		if errors.Is(err, ErrNoToken) {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			unauthorized(w, r, err)
			return
		}
		p, err := v.Verify(r.Context(), raw)
		if err != nil {
			unauthorized(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

func unauthorized(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="icyre"`)
	msg := "Authentication required"
	if !errors.Is(err, ErrNoToken) {
		msg = "Access token is invalid or expired"
	}
	httpserver.WriteError(w, r, http.StatusUnauthorized, CodeUnauthenticated, msg, nil)
}

// RedisRevocations reads/writes revoked session IDs in Redis under
// "revoked:session:<sid>". Entries live as long as an access token can.
type RedisRevocations struct {
	cl *redis.Client
}

// NewRedisRevocations returns a Revocations backed by cl.
func NewRedisRevocations(cl *redis.Client) *RedisRevocations { return &RedisRevocations{cl: cl} }

func revokedKey(sid string) string { return redis.Key("revoked", "session", sid) }

// IsRevoked implements Revocations.
func (r *RedisRevocations) IsRevoked(ctx context.Context, sid string) (bool, error) {
	n, err := r.cl.Exists(ctx, revokedKey(sid)).Result()
	return n > 0, err
}

// Revoke marks sid revoked for ttl (the access token lifetime).
func (r *RedisRevocations) Revoke(ctx context.Context, sid string, ttl time.Duration) error {
	return r.cl.Set(ctx, revokedKey(sid), 1, ttl).Err()
}
