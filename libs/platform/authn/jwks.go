package authn

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// JWK is one Ed25519 public key in JWKS form (RFC 8037).
type JWK struct {
	Kty string `json:"kty"` // OKP
	Crv string `json:"crv"` // Ed25519
	Kid string `json:"kid"`
	X   string `json:"x"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
}

// JWKS is a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// PublicJWK describes pub as a JWK.
func PublicJWK(kid string, pub ed25519.PublicKey) JWK {
	return JWK{Kty: "OKP", Crv: "Ed25519", Kid: kid, X: base64.RawURLEncoding.EncodeToString(pub), Use: "sig", Alg: "EdDSA"}
}

// StaticKeys is a fixed KeySet (tests, and the issuer verifying its own tokens).
type StaticKeys map[string]ed25519.PublicKey

// Key implements KeySet.
func (s StaticKeys) Key(_ context.Context, kid string) (ed25519.PublicKey, error) {
	if k, ok := s[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key %q", kid)
}

// RemoteKeys fetches and caches the JWKS published by Auth Service. Unknown
// key IDs trigger a refresh (key rotation), rate-limited by minRefresh.
type RemoteKeys struct {
	url        string
	client     *http.Client
	ttl        time.Duration
	minRefresh time.Duration

	mu        sync.Mutex
	keys      map[string]ed25519.PublicKey
	fetchedAt time.Time
}

// NewRemoteKeys returns a JWKS-backed KeySet.
func NewRemoteKeys(url string, client *http.Client) *RemoteKeys {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return &RemoteKeys{url: url, client: client, ttl: 10 * time.Minute, minRefresh: 30 * time.Second}
}

// Key implements KeySet.
func (r *RemoteKeys) Key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fresh := time.Since(r.fetchedAt) < r.ttl
	if k, ok := r.keys[kid]; ok && fresh {
		return k, nil
	}
	if !r.fetchedAt.IsZero() && time.Since(r.fetchedAt) < r.minRefresh {
		if k, ok := r.keys[kid]; ok {
			return k, nil
		}
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	if err := r.refresh(ctx); err != nil {
		// Keep serving known keys while the JWKS endpoint is unreachable.
		if k, ok := r.keys[kid]; ok {
			return k, nil
		}
		return nil, err
	}
	if k, ok := r.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key %q", kid)
}

func (r *RemoteKeys) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", res.StatusCode)
	}
	var set JWKS
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}
	keys, err := parseJWKS(set)
	if err != nil {
		return err
	}
	r.keys, r.fetchedAt = keys, time.Now()
	return nil
}

func parseJWKS(set JWKS) (map[string]ed25519.PublicKey, error) {
	keys := make(map[string]ed25519.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Kid == "" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("jwks key %q: bad x", k.Kid)
		}
		keys[k.Kid] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks has no Ed25519 keys")
	}
	return keys, nil
}
