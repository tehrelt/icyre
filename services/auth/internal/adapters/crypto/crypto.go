// Package crypto implements password hashing (Argon2id), opaque refresh
// tokens and EdDSA access tokens.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
	"github.com/tehrelt/icyre/services/auth/internal/ports"
)

// Argon2Params are Argon2id cost parameters (OWASP: m=19 MiB, t=2, p=1 minimum).
type Argon2Params struct {
	Memory  uint32 // KiB
	Time    uint32
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

// DefaultArgon2 follows the OWASP recommendation.
var DefaultArgon2 = Argon2Params{Memory: 19 * 1024, Time: 2, Threads: 1, SaltLen: 16, KeyLen: 32}

// Argon2Hasher implements ports.PasswordHasher with PHC-formatted output:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
type Argon2Hasher struct{ p Argon2Params }

// NewArgon2Hasher returns a hasher.
func NewArgon2Hasher(p Argon2Params) *Argon2Hasher { return &Argon2Hasher{p: p} }

var b64 = base64.RawStdEncoding

// Hash implements ports.PasswordHasher.
func (h *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.p.Time, h.p.Memory, h.p.Threads, h.p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.p.Memory, h.p.Time, h.p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify implements ports.PasswordHasher (constant-time comparison; the
// parameters are read from the stored hash so they can be raised later).
func (h *Argon2Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported password hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	var p Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return false, fmt.Errorf("argon2 params: %w", err)
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// RefreshTokens implements ports.RefreshTokens: 256-bit random tokens, stored
// only as SHA-256 hashes (a DB leak does not leak usable tokens).
type RefreshTokens struct{}

// New implements ports.RefreshTokens.
func (RefreshTokens) New() (string, []byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	return token, RefreshTokens{}.Hash(token), nil
}

// Hash implements ports.RefreshTokens.
func (RefreshTokens) Hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SigningKey is the Ed25519 key that signs access tokens.
type SigningKey struct {
	ID      string
	Private ed25519.PrivateKey
}

// ParseSigningKey decodes a base64 (std or URL) 32-byte Ed25519 seed.
func ParseSigningKey(kid, seedB64 string) (SigningKey, error) {
	seed, err := base64.StdEncoding.DecodeString(seedB64)
	if err != nil {
		seed, err = base64.RawURLEncoding.DecodeString(seedB64)
	}
	if err != nil || len(seed) != ed25519.SeedSize {
		return SigningKey{}, errors.New("signing key must be a base64-encoded 32-byte Ed25519 seed")
	}
	return SigningKey{ID: kid, Private: ed25519.NewKeyFromSeed(seed)}, nil
}

// GenerateSigningKey creates an ephemeral key (local development only).
func GenerateSigningKey(kid string) (SigningKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	return SigningKey{ID: kid, Private: priv}, err
}

// Public returns the verification key.
func (k SigningKey) Public() ed25519.PublicKey { return k.Private.Public().(ed25519.PublicKey) }

// JWTIssuer implements ports.TokenIssuer.
type JWTIssuer struct {
	key SigningKey
	ttl time.Duration
}

// NewJWTIssuer returns an issuer of access tokens valid for ttl.
func NewJWTIssuer(key SigningKey, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{key: key, ttl: ttl}
}

// TTL implements ports.TokenIssuer.
func (i *JWTIssuer) TTL() time.Duration { return i.ttl }

// Issue implements ports.TokenIssuer.
func (i *JWTIssuer) Issue(a domain.Account, sessionID uuid.UUID, now time.Time) (ports.AccessToken, error) {
	exp := now.Add(i.ttl)
	claims := authn.Claims{
		SessionID: sessionID.String(),
		Roles:     a.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authn.Issuer,
			Audience:  jwt.ClaimStrings{authn.Audience},
			Subject:   a.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = i.key.ID
	signed, err := tok.SignedString(i.key.Private)
	if err != nil {
		return ports.AccessToken{}, fmt.Errorf("sign access token: %w", err)
	}
	return ports.AccessToken{Token: signed, ExpiresAt: exp}, nil
}

// JWKS publishes the verification key.
func (i *JWTIssuer) JWKS() authn.JWKS {
	return authn.JWKS{Keys: []authn.JWK{authn.PublicJWK(i.key.ID, i.key.Public())}}
}
