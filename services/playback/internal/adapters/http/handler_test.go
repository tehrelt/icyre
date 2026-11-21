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
	"github.com/tehrelt/icyre/services/playback/internal/application"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

type pub struct {
	last   domain.Event
	events []domain.Event
	fail   bool
}

func (p *pub) Publish(_ context.Context, e domain.Event) error {
	if p.fail {
		return errors.New("kafka down")
	}
	p.last = e
	p.events = append(p.events, e)
	return nil
}

type memStore struct {
	sessions map[uuid.UUID]domain.Session
	queues   map[uuid.UUID]domain.Queue
	progress map[string]bool
	fail     bool
}

func newMemStore() *memStore {
	return &memStore{
		sessions: map[uuid.UUID]domain.Session{},
		queues:   map[uuid.UUID]domain.Queue{},
		progress: map[string]bool{},
	}
}

func (m *memStore) GetState(_ context.Context, userID uuid.UUID) (domain.Session, domain.Queue, error) {
	if m.fail {
		return domain.Session{}, domain.Queue{}, errors.New("redis down")
	}
	s, ok := m.sessions[userID]
	if !ok {
		return domain.Session{}, domain.Queue{}, domain.ErrSessionNotFound
	}
	return s, m.queues[userID], nil
}

func (m *memStore) SaveState(_ context.Context, sess domain.Session, q domain.Queue) error {
	if m.fail {
		return errors.New("redis down")
	}
	m.sessions[sess.UserID] = sess
	m.queues[sess.UserID] = q
	return nil
}

func (m *memStore) AllowProgress(_ context.Context, userID, playbackID uuid.UUID, _ time.Duration) (bool, error) {
	if m.fail {
		return false, errors.New("redis down")
	}
	k := userID.String() + ":" + playbackID.String()
	if m.progress[k] {
		return false, nil
	}
	m.progress[k] = true
	return true, nil
}

func signToken(t *testing.T, sk ed25519.PrivateKey, user uuid.UUID) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, authn.Claims{SessionID: "s", RegisteredClaims: jwt.RegisteredClaims{
		Subject: user.String(), Issuer: authn.Issuer, Audience: jwt.ClaimStrings{authn.Audience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}})
	tok.Header["kid"] = "k"
	signed, err := tok.SignedString(sk)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func doReq(mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestReport(t *testing.T) {
	pk, sk, _ := ed25519.GenerateKey(rand.Reader)
	user := uuid.New()
	signed := signToken(t, sk, user)
	p := &pub{}
	mux := http.NewServeMux()
	NewHandler(application.New(p), authn.NewVerifier(authn.StaticKeys{"k": pk}, nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)

	body := `{"type":"finished","playbackId":"` + uuid.NewString() + `","trackId":"` + uuid.NewString() + `","source":"album:abc","durationMs":240000,"listenedMs":239000}`
	if rec := doReq(mux, http.MethodPost, "/api/v1/playback/events", "", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	if rec := doReq(mux, http.MethodPost, "/api/v1/playback/events", signed, body); rec.Code != http.StatusAccepted || p.last.UserID != user || p.last.Kind != domain.Finished {
		t.Fatalf("report: %d %+v", rec.Code, p.last)
	}
	if rec := doReq(mux, http.MethodPost, "/api/v1/playback/events", signed, `{"type":"finished","playbackId":"x","trackId":"y","durationMs":1}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: %d", rec.Code)
	}
	p.fail = true
	if rec := doReq(mux, http.MethodPost, "/api/v1/playback/events", signed, body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("kafka down: %d", rec.Code)
	}
}

func TestSessionAndQueueEndpoints(t *testing.T) {
	pk, sk, _ := ed25519.GenerateKey(rand.Reader)
	user := uuid.New()
	signed := signToken(t, sk, user)
	p := &pub{}
	store := newMemStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := application.New(p).WithStateStore(store, 10*time.Second, log)
	mux := http.NewServeMux()
	NewHandler(svc, authn.NewVerifier(authn.StaticKeys{"k": pk}, nil), log).Register(mux)

	if rec := doReq(mux, http.MethodGet, "/api/v1/playback/session", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous get session: %d", rec.Code)
	}
	if rec := doReq(mux, http.MethodGet, "/api/v1/playback/session", signed, ""); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "SESSION_NOT_FOUND") {
		t.Fatalf("missing session: %d %s", rec.Code, rec.Body.String())
	}

	t1, t2, t3 := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if rec := doReq(mux, http.MethodPost, "/api/v1/playback/sessions", signed, `{"unknown":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json: %d", rec.Code)
	}
	if rec := doReq(mux, http.MethodPost, "/api/v1/playback/sessions", signed, `{"tracks":[]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty tracks: %d", rec.Code)
	}

	rec := doReq(mux, http.MethodPost, "/api/v1/playback/sessions", signed, `{"tracks":["`+t1+`","`+t2+`"],"startIndex":0,"source":"album:a1","durationMs":180000,"shuffle":false,"repeatMode":"all"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start session: %d %s", rec.Code, rec.Body.String())
	}
	var view sessionView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.TrackID == nil || *view.TrackID != t1 || !view.IsPlaying || view.RepeatMode != domain.RepeatAll || len(view.Queue.Tracks) != 2 {
		t.Fatalf("unexpected session view: %+v", view)
	}

	rec = doReq(mux, http.MethodGet, "/api/v1/playback/session", signed, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get session: %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(mux, http.MethodPatch, "/api/v1/playback/state", signed, `{"action":"next","shuffle":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch state: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if !view.Shuffle || view.TrackID == nil || *view.TrackID != t2 {
		t.Fatalf("expected next track t2 with shuffle=true, got %+v", view)
	}

	rec = doReq(mux, http.MethodPut, "/api/v1/playback/queue", signed, `{"tracks":["`+t3+`"],"startIndex":0,"source":"playlist:p1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put queue: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.TrackID == nil || *view.TrackID != t3 || view.Queue.Source != "playlist:p1" {
		t.Fatalf("unexpected queue view: %+v", view)
	}

	store.fail = true
	if rec := doReq(mux, http.MethodGet, "/api/v1/playback/session", signed, ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("redis down: %d", rec.Code)
	}
}
