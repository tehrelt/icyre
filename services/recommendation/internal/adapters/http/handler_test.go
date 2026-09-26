package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/recommendation"
	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/recommendation/internal/application"
)

type fake struct {
	users []uuid.UUID
	err   error
}

func (f *fake) For(_ context.Context, user uuid.UUID) (application.Result, error) {
	f.users = append(f.users, user)
	items := make([]recommendation.Item, 60)
	for i := range items {
		items[i] = recommendation.Item{ID: strconv.Itoa(i), Score: 1}
	}
	source := application.SourcePopular
	if user != uuid.Nil {
		source = application.SourcePersonal
	}
	return application.Result{Source: source, GeneratedAt: time.Unix(0, 0), Tracks: items, Artists: items}, f.err
}

func TestAPI(t *testing.T) {
	pk, sk, _ := ed25519.GenerateKey(rand.Reader)
	user := uuid.New()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s", RegisteredClaims: jwt.RegisteredClaims{
		Subject: user.String(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k"
	signed, _ := tok.SignedString(sk)
	app := &fake{}
	mux := http.NewServeMux()
	NewHandler(app, authn.NewVerifier(authn.StaticKeys{"k": pk}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	get := func(path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	var home homeResponse
	rec := get("/api/v1/recommendations/home", signed)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &home) != nil {
		t.Fatalf("home: %d %s", rec.Code, rec.Body)
	}
	if home.Source != application.SourcePersonal || len(home.Tracks) != homeTracks || len(home.Artists) != homeArtists ||
		app.users[0] != user || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("home %+v users %v", home.meta, app.users)
	}

	var list listResponse
	rec = get("/api/v1/recommendations/tracks?limit=5", "")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Data) != 5 || list.Source != application.SourcePopular {
		t.Fatalf("guest tracks: %d %s", rec.Code, rec.Body)
	}
	if rec := get("/api/v1/recommendations/artists", signed); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Data) != defaultLimit {
		t.Fatalf("artists: %d %s", rec.Code, rec.Body)
	}
	for _, bad := range []string{"0", "51", "x"} {
		if rec := get("/api/v1/recommendations/tracks?limit="+bad, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("limit %s: %d", bad, rec.Code)
		}
	}
	if rec := get("/api/v1/recommendations/home", "not-a-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("invalid token: %d", rec.Code)
	}

	app.err = errors.New("redis down")
	if rec := get("/api/v1/recommendations/home", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("store down: %d", rec.Code)
	}
}
