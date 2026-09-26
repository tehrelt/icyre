package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestTracksChunksAndMaps(t *testing.T) {
	album, artist := uuid.New(), uuid.New()
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		if r.URL.Path != "/api/v1/tracks" || len(ids) > maxIDs {
			t.Errorf("request %s with %d ids", r.URL.Path, len(ids))
		}
		data := []map[string]any{}
		for _, id := range ids[:1] { // the rest are unknown
			data = append(data, map[string]any{"id": id, "albumId": album.String(), "artistIds": []string{artist.String()}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()

	ids := make([]uuid.UUID, 150)
	for i := range ids {
		ids[i] = uuid.New()
	}
	got, err := New(srv.URL, srv.Client()).Tracks(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(got) != 2 || got[ids[0]].AlbumID != album || got[ids[100]].ArtistIDs[0] != artist {
		t.Fatalf("requests %d got %v", requests, got)
	}
}

func TestTracksFailsOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if _, err := New(srv.URL, srv.Client()).Tracks(context.Background(), []uuid.UUID{uuid.New()}); err == nil {
		t.Fatal("want error")
	}
}
