//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestPlaybackFlow (TASK-037.8): a new listener registers, logs in, reports a
// playback (started → finished) and the listen reaches their history via
// Kafka (playback.events → Listening History).
func TestPlaybackFlow(t *testing.T) {
	e := setup(t)
	f := e.createCatalog(t, suffix(t), 200_000)

	email, password, _ := e.register(t)
	tok := e.login(t, email, password)

	// Telemetry and history require a token.
	if status, body := e.do(t, http.MethodGet, e.gateway+"/api/v1/me/history/tracks", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("anonymous history: status %d, want 401: %s", status, body)
	}

	var empty historyPage
	e.call(t, http.MethodGet, e.gateway+"/api/v1/me/history/tracks", tok.AccessToken, nil, http.StatusOK, &empty)
	if len(empty.Data) != 0 {
		t.Fatalf("a new listener has history: %+v", empty.Data)
	}

	playbackID := newUUID(t)
	source := "album:" + f.album.ID
	for _, ev := range []map[string]any{
		{"type": "started", "playbackId": playbackID, "trackId": f.track.ID, "source": source, "durationMs": 200_000, "listenedMs": 0},
		{"type": "finished", "playbackId": playbackID, "trackId": f.track.ID, "source": source, "durationMs": 200_000, "listenedMs": 200_000},
	} {
		e.call(t, http.MethodPost, e.gateway+"/api/v1/playback/events", tok.AccessToken, ev, http.StatusAccepted, nil)
	}
	if status, body := e.do(t, http.MethodPost, e.gateway+"/api/v1/playback/events", tok.AccessToken, map[string]any{
		"type": "paused", "playbackId": playbackID, "trackId": f.track.ID, "durationMs": 200_000,
	}); status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid event: status %d, want 422: %s", status, body)
	}

	e.eventually(t, "listen in history", func() (bool, string) {
		var page historyPage
		e.call(t, http.MethodGet, e.gateway+"/api/v1/me/history/tracks", tok.AccessToken, nil, http.StatusOK, &page)
		for _, l := range page.Data {
			if l.TrackID == f.track.ID {
				ok := l.ListenedMs == 200_000 && l.Source != nil && *l.Source == source
				return ok, fmt.Sprintf("%+v", l)
			}
		}
		return false, fmt.Sprintf("%d listens", len(page.Data))
	})
}

type historyPage struct {
	Data []struct {
		TrackID    string  `json:"trackId"`
		Source     *string `json:"source"`
		ListenedMs int64   `json:"listenedMs"`
	} `json:"data"`
}
