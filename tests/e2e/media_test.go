//go:build e2e

package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestMediaUploadFlow (TASK-037.7): an artist opens an upload session, PUTs
// the file straight to object storage with the presigned URL, completes the
// session, and the pipeline (media.uploaded → Catalog PROCESSING →
// Transcoder → media.transcoded → Catalog READY) publishes the track.
func TestMediaUploadFlow(t *testing.T) {
	e := setup(t)
	if e.dsn == "" {
		t.Skip("E2E_DATABASE_DSN is not set: needed to grant the ARTIST role")
	}
	const durationMs = 3_000
	f := e.createCatalog(t, suffix(t), durationMs)

	email, password, tok := e.register(t)
	// A listener may not upload.
	if status, body := e.do(t, http.MethodPost, e.gateway+"/api/v1/media/uploads", tok.AccessToken, map[string]any{
		"trackId": f.track.ID, "contentType": "audio/wav", "sizeBytes": 1, "sha256": hex.EncodeToString(make([]byte, 32)),
	}); status != http.StatusForbidden {
		t.Fatalf("listener upload: status %d, want 403: %s", status, body)
	}
	e.grantArtist(t, tok.User.ID)
	tok = e.login(t, email, password)

	audio := wav(durationMs)
	sum := sha256.Sum256(audio)
	var up struct {
		ID      string `json:"id"`
		TrackID string `json:"trackId"`
		Status  string `json:"status"`
		Upload  struct {
			Method  string            `json:"method"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"upload"`
	}
	e.call(t, http.MethodPost, e.gateway+"/api/v1/media/uploads", tok.AccessToken, map[string]any{
		"trackId": f.track.ID, "contentType": "audio/wav", "sizeBytes": len(audio), "sha256": hex.EncodeToString(sum[:]),
	}, http.StatusCreated, &up)
	if up.Status != "PENDING" || up.TrackID != f.track.ID || up.Upload.URL == "" {
		t.Fatalf("upload session %+v", up)
	}

	// Completing before the bytes are there is refused.
	if status, body := e.do(t, http.MethodPost, e.gateway+"/api/v1/media/uploads/"+up.ID+"/complete", tok.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("early complete: status %d, want 409: %s", status, body)
	}

	put(t, e.http, up.Upload.Method, up.Upload.URL, up.Upload.Headers, audio)

	var done struct {
		Status        string  `json:"status"`
		FailureReason *string `json:"failureReason"`
	}
	e.call(t, http.MethodPost, e.gateway+"/api/v1/media/uploads/"+up.ID+"/complete", tok.AccessToken, nil, http.StatusOK, &done)
	if done.Status != "COMPLETED" {
		t.Fatalf("complete: status %q, reason %v", done.Status, done.FailureReason)
	}

	e.eventually(t, "track READY after transcoding", func() (bool, string) {
		s := e.trackStatus(t, f.track.ID)
		return s == "READY", s
	})
}

// grantArtist gives the account the ARTIST role. There is no admin API for
// roles yet, so the test goes to the Auth schema directly.
func (e env) grantArtist(t *testing.T, userID string) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), e.dsn)
	if err != nil {
		t.Fatalf("connect E2E_DATABASE_DSN: %v", err)
	}
	defer func() { _ = conn.Close(t.Context()) }()
	tag, err := conn.Exec(t.Context(),
		`UPDATE auth.accounts SET roles = array_append(roles, 'ARTIST'), updated_at = now() WHERE id = $1 AND NOT 'ARTIST' = ANY(roles)`, userID)
	if err != nil {
		t.Fatalf("grant ARTIST: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("grant ARTIST: %d rows updated", tag.RowsAffected())
	}
}

// put uploads the body with the presigned request, as a browser would.
func put(t *testing.T, cl *http.Client, method, url string, headers map[string]string, body []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := cl.Do(req)
	if err != nil {
		t.Fatalf("presigned %s: %v", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("presigned %s: status %d: %s", method, resp.StatusCode, raw)
	}
}

// wav renders a 16-bit mono 44.1 kHz PCM sine tone of the given length.
func wav(durationMs int) []byte {
	const rate = 44_100
	n := rate * durationMs / 1000
	data := make([]byte, 2*n)
	for i := range n {
		v := int16(8000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[2*i:], uint16(v))
	}
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + len(data)))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1)) // PCM
	w(uint16(1)) // mono
	w(uint32(rate))
	w(uint32(rate * 2)) // byte rate
	w(uint16(2))        // block align
	w(uint16(16))       // bits per sample
	b.WriteString("data")
	w(uint32(len(data)))
	b.Write(data)
	if b.Len() != 44+len(data) {
		panic(fmt.Sprintf("wav header: %d bytes", b.Len()-len(data)))
	}
	return b.Bytes()
}
