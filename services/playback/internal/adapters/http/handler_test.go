package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/playback/internal/application"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

type pub struct {
	last domain.Event
	fail bool
}

func (p *pub) Publish(_ context.Context, e domain.Event) error {
	if p.fail {
		return errors.New("kafka down")
	}
	p.last = e
	return nil
}

func TestReport(t *testing.T) {
	pk, sk, _ := ed25519.GenerateKey(rand.Reader)
	user := uuid.New()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s", RegisteredClaims: jwt.RegisteredClaims{
		Subject: user.String(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k"
	signed, _ := tok.SignedString(sk)
	p := &pub{}
	mux := http.NewServeMux()
	NewHandler(application.New(p), authn.NewVerifier(authn.StaticKeys{"k": pk}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)

	post := func(token, body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/playback/events", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	body := `{"type":"finished","playbackId":"` + uuid.NewString() + `","trackId":"` + uuid.NewString() + `","source":"album:abc","durationMs":240000,"listenedMs":239000}`
	if code := post("", body); code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", code)
	}
	if code := post(signed, body); code != http.StatusAccepted || p.last.UserID != user || p.last.Kind != domain.Finished {
		t.Fatalf("report: %d %+v", code, p.last)
	}
	if code := post(signed, `{"type":"finished","playbackId":"x","trackId":"y","durationMs":1}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: %d", code)
	}
	p.fail = true
	if code := post(signed, body); code != http.StatusServiceUnavailable {
		t.Fatalf("kafka down: %d", code)
	}
}
