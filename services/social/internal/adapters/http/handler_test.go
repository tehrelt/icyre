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
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/social/internal/application"
	"github.com/tehrelt/icyre/services/social/internal/domain"
)

type fakeSocial struct {
	followed map[domain.Target]time.Time
	down     bool
}

func (f *fakeSocial) Follow(_ context.Context, user uuid.UUID, t domain.Target) (domain.Follow, error) {
	switch {
	case t.ID == uuid.Nil:
		return domain.Follow{}, domain.ErrNotFound
	case t.Type == domain.TargetUser && t.ID == user:
		return domain.Follow{}, domain.ErrSelfFollow
	}
	f.followed[t] = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	return domain.Follow{Target: t}, nil
}
func (f *fakeSocial) Unfollow(_ context.Context, _ uuid.UUID, t domain.Target) error {
	delete(f.followed, t)
	return nil
}
func (f *fakeSocial) Followers(_ context.Context, t domain.Target, _ *domain.Cursor, _ int) (application.Page, error) {
	if f.down {
		return application.Page{}, errors.New("db down")
	}
	return application.Page{Follows: []domain.Follow{{FollowerID: t.ID, FollowedAt: time.Unix(0, 0)}}}, nil
}
func (f *fakeSocial) Following(_ context.Context, _ uuid.UUID, typ domain.TargetType, _ *domain.Cursor, _ int) (application.Page, error) {
	var p application.Page
	for t, at := range f.followed {
		if t.Type == typ {
			p.Follows = append(p.Follows, domain.Follow{Target: t, FollowedAt: at})
		}
	}
	p.Next = &domain.Cursor{At: time.Unix(1, 0), ID: uuid.New()}
	return p, nil
}
func (f *fakeSocial) Contains(_ context.Context, _ uuid.UUID, typ domain.TargetType, ids []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, id := range ids {
		if _, ok := f.followed[domain.Target{Type: typ, ID: id}]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
func (f *fakeSocial) Counts(context.Context, domain.Target) (domain.Counts, error) {
	return domain.Counts{Followers: 3, FollowingUsers: 1, FollowingArtists: 2}, nil
}

func setup(t *testing.T) (http.Handler, *fakeSocial, string, uuid.UUID) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	app := &fakeSocial{followed: map[domain.Target]time.Time{}}
	mux := http.NewServeMux()
	NewHandler(app, authn.NewVerifier(authn.StaticKeys{"k1": pub}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	me := uuid.New()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s1", RegisteredClaims: jwt.RegisteredClaims{
		Subject: me.String(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return mux, app, s, me
}

func do(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFollowAPI(t *testing.T) {
	h, app, tok, me := setup(t)
	artist, friend := uuid.New(), uuid.New()
	if rec := do(h, http.MethodPut, "/api/v1/artists/"+artist.String()+"/follow", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	for range 2 {
		if rec := do(h, http.MethodPut, "/api/v1/artists/"+artist.String()+"/follow", tok); rec.Code != http.StatusNoContent {
			t.Fatalf("follow artist: %d %s", rec.Code, rec.Body)
		}
	}
	if rec := do(h, http.MethodPut, "/api/v1/users/"+friend.String()+"/follow", tok); rec.Code != http.StatusNoContent {
		t.Fatalf("follow user: %d", rec.Code)
	}
	for path, code := range map[string]string{
		"/api/v1/users/" + uuid.Nil.String() + "/follow":   "USER_NOT_FOUND",
		"/api/v1/artists/" + uuid.Nil.String() + "/follow": "ARTIST_NOT_FOUND",
		"/api/v1/users/" + me.String() + "/follow":         "CANNOT_FOLLOW_SELF",
	} {
		if rec := do(h, http.MethodPut, path, tok); !strings.Contains(rec.Body.String(), code) {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := do(h, http.MethodPut, "/api/v1/users/nope/follow", tok); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id: %d", rec.Code)
	}

	rec := do(h, http.MethodGet, "/api/v1/users/"+me.String()+"/following?type=artist&limit=10", "")
	var list listResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Data) != 1 || list.Data[0].ArtistID != artist.String() || !list.Pagination.HasMore {
		t.Fatalf("following: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodGet, "/api/v1/artists/"+artist.String()+"/followers", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"userId":"`+artist.String()) || !strings.Contains(rec.Body.String(), `"hasMore":false`) {
		t.Fatalf("followers: %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodGet, "/api/v1/me/following/contains?type=artist&ids="+artist.String()+","+uuid.NewString(), tok)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"data":["`+artist.String()+`"]}`+"\n" || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("contains: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/api/v1/users/"+me.String()+"/follow-counts", ""); rec.Body.String() != `{"followers":3,"followingUsers":1,"followingArtists":2}`+"\n" {
		t.Fatalf("user counts: %s", rec.Body)
	}
	if rec := do(h, http.MethodGet, "/api/v1/artists/"+artist.String()+"/follow-counts", ""); rec.Body.String() != `{"followers":3}`+"\n" {
		t.Fatalf("artist counts: %s", rec.Body)
	}
	for range 2 {
		if rec := do(h, http.MethodDelete, "/api/v1/artists/"+artist.String()+"/follow", tok); rec.Code != http.StatusNoContent {
			t.Fatalf("unfollow: %d", rec.Code)
		}
	}
	if len(app.followed) != 1 {
		t.Fatalf("after unfollow: %v", app.followed)
	}

	for path, want := range map[string]int{
		"/api/v1/users/" + me.String() + "/following?type=album":       http.StatusUnprocessableEntity,
		"/api/v1/users/" + me.String() + "/following?limit=101":        http.StatusUnprocessableEntity,
		"/api/v1/users/" + me.String() + "/followers?cursor=abc":       http.StatusUnprocessableEntity,
		"/api/v1/me/following/contains?ids=":                           http.StatusUnprocessableEntity,
		"/api/v1/me/following/contains?ids=zz":                         http.StatusUnprocessableEntity,
		"/api/v1/me/following/contains?type=x&ids=" + uuid.NewString(): http.StatusUnprocessableEntity,
	} {
		if rec := do(h, http.MethodGet, path, tok); rec.Code != want {
			t.Errorf("%s: %d want %d", path, rec.Code, want)
		}
	}
	app.down = true
	if rec := do(h, http.MethodGet, "/api/v1/users/"+me.String()+"/followers", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("db down: %d", rec.Code)
	}
}
