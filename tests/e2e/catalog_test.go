//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// TestCatalogFlow (TASK-037.6): content created in Catalog is readable through
// the gateway, the gateway keeps the catalogue read-only, and the Search
// Indexer projects it into OpenSearch (artist at once, track once READY).
func TestCatalogFlow(t *testing.T) {
	e := setup(t)
	name := suffix(t)
	f := e.createCatalog(t, name, 185_000)

	// Read back through the gateway.
	var gotArtist artist
	e.call(t, http.MethodGet, e.gateway+"/api/v1/artists/"+f.artist.ID, "", nil, http.StatusOK, &gotArtist)
	if gotArtist.Name != "Artist "+name {
		t.Fatalf("artist name %q", gotArtist.Name)
	}
	var gotAlbum album
	e.call(t, http.MethodGet, e.gateway+"/api/v1/albums/"+f.album.ID, "", nil, http.StatusOK, &gotAlbum)
	if gotAlbum.Title != "Album "+name || len(gotAlbum.ArtistIDs) != 1 || gotAlbum.ArtistIDs[0] != f.artist.ID {
		t.Fatalf("album %+v", gotAlbum)
	}
	var albumTracks struct {
		Data []track `json:"data"`
	}
	e.call(t, http.MethodGet, e.gateway+"/api/v1/albums/"+f.album.ID+"/tracks", "", nil, http.StatusOK, &albumTracks)
	if len(albumTracks.Data) != 1 || albumTracks.Data[0].ID != f.track.ID {
		t.Fatalf("album tracks %+v", albumTracks.Data)
	}
	var artistAlbums struct {
		Data []album `json:"data"`
	}
	e.call(t, http.MethodGet, e.gateway+"/api/v1/artists/"+f.artist.ID+"/albums", "", nil, http.StatusOK, &artistAlbums)
	if len(artistAlbums.Data) != 1 || artistAlbums.Data[0].ID != f.album.ID {
		t.Fatalf("artist albums %+v", artistAlbums.Data)
	}

	// Writes are not exposed to clients.
	if status, body := e.do(t, http.MethodPost, e.gateway+"/api/v1/artists", "", map[string]string{"name": "nope"}); status != http.StatusForbidden {
		t.Fatalf("POST artists through the gateway: status %d, want 403: %s", status, body)
	}

	// The artist is searchable as soon as the indexer has consumed the event.
	e.eventually(t, "artist in search", func() (bool, string) {
		res := e.search(t, "artists", name)
		for _, a := range res.Artists {
			if a.ID == f.artist.ID {
				return true, ""
			}
		}
		return false, fmt.Sprintf("%d artists", len(res.Artists))
	})

	// A DRAFT track is not searchable; publishing it (the transitions the
	// media pipeline drives) makes it appear as available.
	for _, status := range []string{"PROCESSING", "READY"} {
		var tr track
		e.call(t, http.MethodPatch, e.catalog+"/api/v1/tracks/"+f.track.ID, "", map[string]string{"status": status}, http.StatusOK, &tr)
		if tr.Status != status {
			t.Fatalf("track status %q, want %q", tr.Status, status)
		}
	}
	if got := e.trackStatus(t, f.track.ID); got != "READY" {
		t.Fatalf("track status through the gateway %q", got)
	}
	e.eventually(t, "track in search", func() (bool, string) {
		res := e.search(t, "tracks", name)
		for _, tr := range res.Tracks {
			if tr.ID == f.track.ID {
				return tr.Available && tr.AlbumID == f.album.ID, fmt.Sprintf("%+v", tr)
			}
		}
		return false, fmt.Sprintf("%d tracks", len(res.Tracks))
	})
}

type searchResult struct {
	Tracks []struct {
		ID        string `json:"id"`
		AlbumID   string `json:"albumId"`
		Available bool   `json:"available"`
	} `json:"tracks"`
	Artists []struct {
		ID string `json:"id"`
	} `json:"artists"`
}

func (e env) search(t *testing.T, typ, q string) searchResult {
	t.Helper()
	var res searchResult
	e.call(t, http.MethodGet, e.gateway+"/api/v1/search?"+url.Values{"q": {q}, "type": {typ}}.Encode(), "", nil, http.StatusOK, &res)
	return res
}
