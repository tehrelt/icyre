// Package authn verifies ICYRE access tokens and exposes the caller's
// identity to handlers.
//
// Access tokens are short-lived EdDSA (Ed25519) JWTs issued by Auth Service.
// Services verify them locally against the public keys Auth publishes as a
// JWKS; no call to Auth is needed per request.
package authn

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token parameters shared by the issuer (Auth Service) and all verifiers.
const (
	Issuer   = "icyre-auth"
	Audience = "icyre-api"
)

// Roles (specs/security/security.md).
const (
	RoleUser      = "USER"
	RoleArtist    = "ARTIST"
	RoleModerator = "MODERATOR"
	RoleAdmin     = "ADMIN"
)

// Errors returned by Verify.
var (
	ErrNoToken      = errors.New("authn: no bearer token")
	ErrInvalidToken = errors.New("authn: invalid token")
	ErrRevoked      = errors.New("authn: session revoked")
)

// Principal is the authenticated caller.
type Principal struct {
	UserID    string
	SessionID string
	Roles     []string
	ExpiresAt time.Time
}

// HasRole reports whether the principal has role.
func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Claims is the access token payload.
type Claims struct {
	SessionID string   `json:"sid"`
	Roles     []string `json:"roles"`
	jwt.RegisteredClaims
}

// KeySet resolves verification keys by key ID.
type KeySet interface {
	Key(ctx context.Context, kid string) (ed25519.PublicKey, error)
}

// Revocations reports sessions revoked before their tokens expire.
type Revocations interface {
	IsRevoked(ctx context.Context, sessionID string) (bool, error)
}

// Verifier validates access tokens.
type Verifier struct {
	keys    KeySet
	revoked Revocations
	leeway  time.Duration
}

// NewVerifier returns a Verifier. revoked may be nil: tokens then stay valid
// until they expire (they are short-lived by design).
func NewVerifier(keys KeySet, revoked Revocations) *Verifier {
	return &Verifier{keys: keys, revoked: revoked, leeway: 30 * time.Second}
}

// Verify parses and validates a compact JWT.
func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		return v.keys.Key(ctx, kid)
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(v.leeway),
	)
	if err != nil || !token.Valid {
		return Principal{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.Subject == "" || claims.SessionID == "" {
		return Principal{}, fmt.Errorf("%w: missing sub or sid", ErrInvalidToken)
	}
	if v.revoked != nil {
		revoked, err := v.revoked.IsRevoked(ctx, claims.SessionID)
		if err != nil {
			// Fail open on a cache outage: the token is signed and short-lived.
			// Auth Service itself re-checks sessions on refresh.
			revoked = false
		}
		if revoked {
			return Principal{}, ErrRevoked
		}
	}
	return Principal{
		UserID:    claims.Subject,
		SessionID: claims.SessionID,
		Roles:     claims.Roles,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

type ctxKey struct{}

// WithPrincipal stores p in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the authenticated principal, if any.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
