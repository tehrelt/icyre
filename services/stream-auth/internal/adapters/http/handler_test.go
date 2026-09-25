package http

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/stream-auth/internal/application"
	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

const (
	readyID   = "01920000-0000-7000-8000-000000000001"
	blockedID = "01920000-0000-7000-8000-000000000002"
	brokenID  = "01920000-0000-7000-8000-000000000003"
)

type fakeApp struct{ last application.Request }

func (f *fakeApp) Authorize(_ context.Context, r application.Request) (domain.Grant, error) {
	f.last = r
	switch r.TrackID {
	case readyID:
		return domain.Grant{TrackID: r.TrackID, Quality: 128, URL: "https://media/x", ExpiresAt: time.Date(2026, 9, 25, 10, 5, 0, 0, time.UTC)}, nil
	case blockedID:
		return domain.Grant{}, domain.ErrTrackBlocked
	case brokenID:
		return domain.Grant{}, errors.New("catalog down")
	}
	return domain.Grant{}, domain.ErrTrackNotFound
}

func setup(t *testing.T) (http.Handler, *fakeApp, string) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	app := &fakeApp{}
	mux := http.NewServeMux()
	NewHandler(app, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil)).Register(mux)
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s1", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "u1", Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return mux, app, s
}

func post(h http.Handler, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/authorize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthorize(t *testing.T) {
	h, app, tok := setup(t)
	if rec := post(h, "", `{"trackId":"`+readyID+`"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	rec := post(h, tok, `{"trackId":"`+readyID+`","quality":"128"}`)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" ||
		rec.Body.String() != `{"trackId":"`+readyID+`","quality":"128","url":"https://media/x","expiresAt":"2026-09-25T10:05:00Z"}`+"\n" {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if app.last.UserID != "u1" || app.last.SessionID != "s1" || app.last.Quality != 128 {
		t.Fatalf("request %+v", app.last)
	}

	cases := map[string]int{
		`{"trackId":"nope"}`:                                 http.StatusUnprocessableEntity,
		`{"trackId":"` + readyID + `","quality":"96"}`:       http.StatusUnprocessableEntity,
		`{"trackId":"` + blockedID + `"}`:                    http.StatusForbidden,
		`{"trackId":"` + brokenID + `"}`:                     http.StatusServiceUnavailable,
		`{"trackId":"01920000-0000-7000-8000-00000000ffff"}`: http.StatusNotFound,
		`{"trackId":1}`:                                      http.StatusBadRequest,
	}
	for body, want := range cases {
		if rec := post(h, tok, body); rec.Code != want {
			t.Errorf("%s: %d, want %d (%s)", body, rec.Code, want, rec.Body)
		}
	}
}
