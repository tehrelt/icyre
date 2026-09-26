//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// env is the stand under test.
type env struct {
	gateway string
	catalog string
	dsn     string
	timeout time.Duration
	http    *http.Client
}

func setup(t *testing.T) env {
	t.Helper()
	base := os.Getenv("E2E_BASE_URL")
	if base == "" {
		t.Skip("E2E_BASE_URL is not set")
	}
	e := env{
		gateway: strings.TrimRight(base, "/"),
		catalog: strings.TrimRight(getenv("E2E_CATALOG_URL", "http://localhost:8081"), "/"),
		dsn:     os.Getenv("E2E_DATABASE_DSN"),
		timeout: 2 * time.Minute,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	if s := os.Getenv("E2E_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			t.Fatalf("E2E_TIMEOUT: %v", err)
		}
		e.timeout = d
	}
	return e
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// suffix makes names unique per run so tests never collide with seed data
// or with each other, and so a search for it has exactly one hit.
func suffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "e2e" + hex.EncodeToString(b)
}

func newUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// call sends a JSON request and decodes a JSON answer into out (when not
// nil), failing the test unless the status is want.
func (e env) call(t *testing.T, method, url, token string, body any, want int, out any) {
	t.Helper()
	status, raw := e.do(t, method, url, token, body)
	if status != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, url, status, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s %s: decode %s: %v", method, url, raw, err)
		}
	}
}

// do sends a JSON request and returns the status and body.
func (e env) do(t *testing.T, method, url, token string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := e.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", method, url, err)
	}
	return resp.StatusCode, raw
}

// eventually polls cond until it reports true or the stand timeout passes;
// cond returns a description of the last state for the failure message.
func (e env) eventually(t *testing.T, what string, cond func() (bool, string)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), e.timeout)
	defer cancel()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := ""
	for {
		ok, state := cond()
		if ok {
			return
		}
		last = state
		select {
		case <-ctx.Done():
			t.Fatalf("%s: not reached within %s (last: %s)", what, e.timeout, last)
		case <-tick.C:
		}
	}
}

type tokens struct {
	AccessToken string `json:"accessToken"`
	User        struct {
		ID    string   `json:"id"`
		Roles []string `json:"roles"`
	} `json:"user"`
}

// register creates a fresh listener account through the gateway.
func (e env) register(t *testing.T) (email, password string, tok tokens) {
	t.Helper()
	email = suffix(t) + "@e2e.icyre.test"
	password = "e2e-" + suffix(t)
	e.call(t, http.MethodPost, e.gateway+"/api/v1/auth/register", "", map[string]string{"email": email, "password": password}, http.StatusCreated, &tok)
	if tok.AccessToken == "" || tok.User.ID == "" {
		t.Fatalf("register: no access token or user id: %+v", tok)
	}
	return email, password, tok
}

func (e env) login(t *testing.T, email, password string) tokens {
	t.Helper()
	var tok tokens
	e.call(t, http.MethodPost, e.gateway+"/api/v1/auth/login", "", map[string]string{"email": email, "password": password}, http.StatusOK, &tok)
	return tok
}

// Catalog fixtures. Writes go straight to the Catalog Service: the gateway
// exposes the catalogue read-only.

type artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type album struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	AlbumType string   `json:"albumType"`
	ArtistIDs []string `json:"artistIds"`
}

type track struct {
	ID         string   `json:"id"`
	AlbumID    string   `json:"albumId"`
	ArtistIDs  []string `json:"artistIds"`
	Title      string   `json:"title"`
	DurationMs int64    `json:"durationMs"`
	Status     string   `json:"status"`
}

type catalogFixture struct {
	artist artist
	album  album
	track  track
}

func (e env) createCatalog(t *testing.T, name string, durationMs int64) catalogFixture {
	t.Helper()
	var f catalogFixture
	e.call(t, http.MethodPost, e.catalog+"/api/v1/artists", "", map[string]string{"name": "Artist " + name}, http.StatusCreated, &f.artist)
	e.call(t, http.MethodPost, e.catalog+"/api/v1/albums", "", map[string]any{
		"title": "Album " + name, "albumType": "SINGLE", "releaseDate": "2026-01-02", "artistIds": []string{f.artist.ID},
	}, http.StatusCreated, &f.album)
	e.call(t, http.MethodPost, e.catalog+"/api/v1/tracks", "", map[string]any{
		"albumId": f.album.ID, "artistIds": []string{f.artist.ID}, "title": "Track " + name,
		"durationMs": durationMs, "trackNumber": 1, "discNumber": 1,
	}, http.StatusCreated, &f.track)
	if f.track.Status != "DRAFT" {
		t.Fatalf("new track status %q, want DRAFT", f.track.Status)
	}
	return f
}

// trackStatus reads a track through the gateway.
func (e env) trackStatus(t *testing.T, id string) string {
	t.Helper()
	var tr track
	e.call(t, http.MethodGet, e.gateway+"/api/v1/tracks/"+id, "", nil, http.StatusOK, &tr)
	return tr.Status
}
