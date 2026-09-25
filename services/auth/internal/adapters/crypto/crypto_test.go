package crypto

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/auth/internal/domain"
)

func TestArgon2(t *testing.T) {
	h := NewArgon2Hasher(Argon2Params{Memory: 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32})
	enc, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$m=1024,t=1,p=1$") || strings.Contains(enc, "correct horse") {
		t.Fatalf("encoded = %s", enc)
	}
	if ok, err := h.Verify("correct horse battery", enc); err != nil || !ok {
		t.Fatalf("verify = %v %v", ok, err)
	}
	if ok, _ := h.Verify("wrong", enc); ok {
		t.Fatal("wrong password verified")
	}
	other, _ := h.Hash("correct horse battery")
	if other == enc {
		t.Fatal("salts must differ")
	}
	if _, err := h.Verify("x", "$bcrypt$..."); err == nil {
		t.Fatal("expected format error")
	}
}

func TestRefreshTokens(t *testing.T) {
	tok, hash, err := RefreshTokens{}.New()
	if err != nil || len(tok) < 40 {
		t.Fatalf("token %q err %v", tok, err)
	}
	if !bytes.Equal(RefreshTokens{}.Hash(tok), hash) {
		t.Fatal("hash mismatch")
	}
	tok2, _, _ := RefreshTokens{}.New()
	if tok == tok2 {
		t.Fatal("tokens must be random")
	}
}

func TestIssuedTokensVerifyWithPlatformVerifier(t *testing.T) {
	key, err := GenerateSigningKey("k1")
	if err != nil {
		t.Fatal(err)
	}
	iss := NewJWTIssuer(key, 15*time.Minute)
	acc := domain.NewAccount(uuid.New(), "rin@example.com", "x", time.Now())
	sid := uuid.New()
	at, err := iss.Issue(acc, sid, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	jwks := iss.JWKS()
	if len(jwks.Keys) != 1 || jwks.Keys[0].Kid != "k1" || jwks.Keys[0].Crv != "Ed25519" {
		t.Fatalf("jwks = %+v", jwks)
	}
	v := authn.NewVerifier(authn.StaticKeys{"k1": key.Public()}, nil)
	p, err := v.Verify(context.Background(), at.Token)
	if err != nil || p.UserID != acc.ID.String() || p.SessionID != sid.String() || !p.HasRole(authn.RoleUser) {
		t.Fatalf("principal = %+v err = %v", p, err)
	}
}

func TestParseSigningKey(t *testing.T) {
	if _, err := ParseSigningKey("k", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSigningKey("k", "short"); err == nil {
		t.Fatal("expected error")
	}
}
