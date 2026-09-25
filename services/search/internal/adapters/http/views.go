package http

import (
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"github.com/tehrelt/icyre/libs/contracts/search"
	"github.com/tehrelt/icyre/services/search/internal/application"
)

// Response shapes of GET /api/v1/search: the same entity views the web
// client uses on BFF pages (apps/web entities/*).

type trackView struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ArtistName string  `json:"artistName"`
	AlbumID    string  `json:"albumId"`
	AlbumTitle string  `json:"albumTitle"`
	DurationS  int     `json:"durationSec"`
	Explicit   bool    `json:"explicit"`
	CoverURL   *string `json:"coverUrl"`
	Art        int     `json:"art"`
	Liked      bool    `json:"liked"`
	Available  bool    `json:"available"`
}

type artistView struct {
	Kind     string  `json:"kind"`
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	CoverURL *string `json:"coverUrl"`
	Art      int     `json:"art"`
}

type albumView struct {
	Kind       string  `json:"kind"`
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ArtistName string  `json:"artistName"`
	Year       *int    `json:"year,omitempty"`
	AlbumType  string  `json:"albumType,omitempty"`
	Explicit   bool    `json:"explicit"`
	CoverURL   *string `json:"coverUrl"`
	Art        int     `json:"art"`
}

type playlistView struct {
	Kind       string  `json:"kind"`
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Owner      string  `json:"owner,omitempty"`
	TrackCount int     `json:"trackCount"`
	CoverURL   *string `json:"coverUrl"`
	Art        int     `json:"art"`
}

type topResultView struct {
	Item     any  `json:"item"`
	Verified bool `json:"verified"`
}

type countsView struct {
	Tracks    int `json:"tracks"`
	Artists   int `json:"artists"`
	Albums    int `json:"albums"`
	Playlists int `json:"playlists"`
}

type searchResponse struct {
	Query      string         `json:"query"`
	Type       string         `json:"type"`
	Counts     countsView     `json:"counts"`
	TopResult  *topResultView `json:"topResult"`
	Tracks     []trackView    `json:"tracks"`
	Artists    []artistView   `json:"artists"`
	Albums     []albumView    `json:"albums"`
	Playlists  []playlistView `json:"playlists"`
	DidYouMean *string        `json:"didYouMean"`
}

type suggestionView struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Text     string `json:"text"`
	Subtitle string `json:"subtitle,omitempty"`
}

type suggestResponse struct {
	Query       string           `json:"query"`
	Suggestions []suggestionView `json:"suggestions"`
}

// artIndex picks the Design System fallback cover (0–7), the same way the
// BFF does, so an album looks the same on every screen.
func artIndex(id string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return int(h.Sum32() % 8)
}

func year(date string) *int {
	if len(date) < 4 {
		return nil
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return nil
	}
	return &y
}

func track(t search.Track) trackView {
	return trackView{
		ID: t.ID, Title: t.Title, ArtistName: strings.Join(t.ArtistNames, ", "), AlbumID: t.AlbumID, AlbumTitle: t.AlbumTitle,
		DurationS: int(math.Round(float64(t.DurationMs) / 1000)), Explicit: t.Explicit, Art: artIndex(t.AlbumID), Available: t.Available,
	}
}

func artist(a search.Artist) artistView {
	return artistView{Kind: "artist", ID: a.ID, Name: a.Name, Art: artIndex(a.ID)}
}

func album(a search.Album) albumView {
	return albumView{Kind: "album", ID: a.ID, Title: a.Title, ArtistName: strings.Join(a.ArtistNames, ", "), Year: year(a.ReleaseDate), AlbumType: a.AlbumType, Art: artIndex(a.ID)}
}

func playlist(p search.Playlist) playlistView {
	return playlistView{Kind: "playlist", ID: p.ID, Title: p.Title, Owner: p.OwnerName, TrackCount: p.TrackCount, Art: artIndex(p.ID)}
}

func mapHits[T, V any](hits []application.Hit[T], f func(T) V) []V {
	out := make([]V, len(hits))
	for i, h := range hits {
		out[i] = f(h.Doc)
	}
	return out
}

func toResponse(r application.Result) searchResponse {
	p := r.Page
	res := searchResponse{
		Query: r.Query, Type: string(r.Type),
		Counts:    countsView{Tracks: p.Counts.Tracks, Artists: p.Counts.Artists, Albums: p.Counts.Albums, Playlists: p.Counts.Playlists},
		Tracks:    mapHits(p.Tracks, track),
		Artists:   mapHits(p.Artists, artist),
		Albums:    mapHits(p.Albums, album),
		Playlists: mapHits(p.Playlists, playlist),
	}
	switch r.TopKind {
	case application.TopArtist:
		a := p.Artists[0].Doc
		res.TopResult = &topResultView{Item: artist(a), Verified: a.Verified}
	case application.TopAlbum:
		res.TopResult = &topResultView{Item: album(p.Albums[0].Doc)}
	case application.TopPlaylist:
		res.TopResult = &topResultView{Item: playlist(p.Playlists[0].Doc)}
	}
	if r.DidYouMean != "" {
		res.DidYouMean = &r.DidYouMean
	}
	return res
}
