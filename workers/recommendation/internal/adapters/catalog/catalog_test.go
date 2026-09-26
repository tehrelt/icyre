package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestTracksPagesAlbumsAndKeepsReadyTracks(t *testing.T) {
	a1, a2, genre, artist := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	ready, draft := uuid.NewString(), uuid.NewString()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body any
		switch {
		case r.URL.Path == "/api/v1/albums" && r.URL.Query().Get("cursor") == "":
			body = map[string]any{"data": []any{map[string]any{"id": a1, "releaseDate": "2026-09-01", "genreIds": []string{genre}}},
				"pagination": map[string]any{"nextCursor": "c2", "hasMore": true}}
		case r.URL.Path == "/api/v1/albums":
			body = map[string]any{"data": []any{map[string]any{"id": a2, "releaseDate": ""}}, "pagination": map[string]any{"hasMore": false}}
		case r.URL.Path == "/api/v1/albums/"+a1+"/tracks":
			body = map[string]any{"data": []any{
				map[string]any{"id": ready, "artistIds": []string{artist}, "status": "READY"},
				map[string]any{"id": draft, "artistIds": []string{artist}, "status": "DRAFT"},
			}}
		default:
			body = map[string]any{"data": []any{}}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	tracks, err := New(srv.URL, srv.Client()).Tracks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].ID.String() != ready || tracks[0].AlbumID.String() != a1 ||
		tracks[0].GenreIDs[0].String() != genre || tracks[0].Released.Day() != 1 || tracks[0].ArtistIDs[0].String() != artist {
		t.Fatalf("tracks %+v", tracks)
	}
}
