package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	"github.com/tehrelt/icyre/services/history/internal/application"
	"github.com/tehrelt/icyre/services/history/internal/domain"
)

type fake struct{}

func (fake) Tracks(context.Context, uuid.UUID, *domain.Cursor, int) (application.Page, error) {
	return application.Page{Listens: []domain.Listen{{PlaybackID: uuid.New(), TrackID: uuid.Nil, ListenedMs: 1, PlayedAt: time.Unix(0, 0)}}}, nil
}
func (fake) RecentSources(context.Context, uuid.UUID, int) ([]domain.RecentSource, error) {
	return []domain.RecentSource{{Source: "album:a", PlayedAt: time.Unix(0, 0)}}, nil
}

func TestHistoryAPI(t *testing.T) {
	pk, sk, _ := ed25519.GenerateKey(rand.Reader)
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s", RegisteredClaims: jwt.RegisteredClaims{
		Subject: uuid.NewString(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k"
	signed, _ := tok.SignedString(sk)
	mux := http.NewServeMux()
	NewHandler(fake{}, authn.NewVerifier(authn.StaticKeys{"k": pk}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	get := func(path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	if rec := get("/api/v1/me/history/tracks", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	if rec := get("/api/v1/me/history/tracks", signed); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"source":null`) {
		t.Fatalf("tracks: %d %s", rec.Code, rec.Body)
	}
	if rec := get("/api/v1/me/history/sources?limit=5", signed); !strings.Contains(rec.Body.String(), `"source":"album:a"`) {
		t.Fatalf("sources: %s", rec.Body)
	}
	for _, p := range []string{"/api/v1/me/history/tracks?limit=0", "/api/v1/me/history/tracks?cursor=zz", "/api/v1/me/history/sources?limit=99"} {
		if rec := get(p, signed); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
}
