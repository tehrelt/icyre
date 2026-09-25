// Package views holds the page contracts the BFF serves to the web client.
//
// A BFF has no domain of its own: these page-shaped view models ARE its
// product, so they carry the public JSON names. They mirror the zod schemas
// in apps/web (pages/home/api/homeFeed.ts, pages/album/api/albumPage.ts).
package views

// Track is a playable row.
type Track struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	ArtistName  string  `json:"artistName"`
	AlbumID     string  `json:"albumId"`
	AlbumTitle  string  `json:"albumTitle"`
	DurationSec int     `json:"durationSec"`
	Explicit    bool    `json:"explicit"`
	CoverURL    *string `json:"coverUrl"`
	Art         int     `json:"art"`
	Liked       bool    `json:"liked"`
	Available   bool    `json:"available"`
}

// AlbumCard is an album in a grid.
type AlbumCard struct {
	Kind       string  `json:"kind"` // always "album"
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ArtistName string  `json:"artistName"`
	Year       int     `json:"year,omitempty"`
	Explicit   bool    `json:"explicit"`
	CoverURL   *string `json:"coverUrl"`
	Art        int     `json:"art"`
	AlbumType  string  `json:"albumType,omitempty"`
	Badge      string  `json:"badge,omitempty"`
}

// AlbumPage is GET /api/v1/pages/albums/{id}.
type AlbumPage struct {
	Album        AlbumHeader `json:"album"`
	Artist       ArtistRef   `json:"artist"`
	Tracks       []Track     `json:"tracks"`
	MoreByArtist []AlbumCard `json:"moreByArtist"`
	// Unavailable names sections that could not be filled (graceful degradation).
	Unavailable []string `json:"unavailable,omitempty"`
}

// AlbumHeader is the hero of the album page.
type AlbumHeader struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	AlbumType   string   `json:"albumType"`
	Year        int      `json:"year"`
	ReleaseDate string   `json:"releaseDate"`
	TrackCount  int      `json:"trackCount"`
	DurationSec int      `json:"durationSec"`
	Tags        []string `json:"tags"`
	HiRes       bool     `json:"hiRes"`
	CoverURL    *string  `json:"coverUrl"`
	Art         int      `json:"art"`
}

// ArtistRef is the credited artist of a page.
type ArtistRef struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl"`
	Art       int     `json:"art"`
}

// Shelf is a titled list of cards.
type Shelf[T any] struct {
	Kicker string `json:"kicker,omitempty"`
	Items  []T    `json:"items"`
}

// Trending holds the two periods of the trending list.
type Trending struct {
	Today []Track `json:"today"`
	Week  []Track `json:"week"`
}

// MadeForYou holds personal mixes.
type MadeForYou struct {
	Featured  any   `json:"featured"`
	Playlists []any `json:"playlists"`
}

// HomePage is GET /api/v1/pages/home.
//
// Sections whose source service does not exist yet (history, editorial,
// recommendations, social) are returned empty and listed in Unavailable.
type HomePage struct {
	RecentlyPlayed  []any            `json:"recentlyPlayed"`
	AlbumOfTheWeek  any              `json:"albumOfTheWeek"`
	Recommended     Shelf[any]       `json:"recommended"`
	NewReleases     Shelf[AlbumCard] `json:"newReleases"`
	Trending        Trending         `json:"trending"`
	MadeForYou      MadeForYou       `json:"madeForYou"`
	FollowedArtists []any            `json:"followedArtists"`
	Unavailable     []string         `json:"unavailable,omitempty"`
}
