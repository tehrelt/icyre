package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

// Request and response DTOs: the public JSON contract (camelCase, UTC
// ISO 8601). They are never passed into the domain directly.

type createArtistRequest struct {
	Name string `json:"name"`
}

type createAlbumRequest struct {
	Title       string   `json:"title"`
	AlbumType   string   `json:"albumType"`
	ReleaseDate string   `json:"releaseDate"`
	ArtistIDs   []string `json:"artistIds"`
	GenreIDs    []string `json:"genreIds"`
}

type createTrackRequest struct {
	AlbumID     string   `json:"albumId"`
	ArtistIDs   []string `json:"artistIds"`
	Title       string   `json:"title"`
	DurationMs  int64    `json:"durationMs"`
	TrackNumber int      `json:"trackNumber"`
	DiscNumber  int      `json:"discNumber"`
	Explicit    bool     `json:"explicit"`
	ISRC        string   `json:"isrc"`
}

type updateTrackRequest struct {
	Title    *string `json:"title"`
	Explicit *bool   `json:"explicit"`
	Status   *string `json:"status"`
}

type artistResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type albumResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	AlbumType   string    `json:"albumType"`
	ReleaseDate string    `json:"releaseDate"`
	ArtistIDs   []string  `json:"artistIds"`
	GenreIDs    []string  `json:"genreIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type trackResponse struct {
	ID          string    `json:"id"`
	AlbumID     string    `json:"albumId"`
	ArtistIDs   []string  `json:"artistIds"`
	Title       string    `json:"title"`
	DurationMs  int64     `json:"durationMs"`
	TrackNumber int       `json:"trackNumber"`
	DiscNumber  int       `json:"discNumber"`
	Explicit    bool      `json:"explicit"`
	ISRC        *string   `json:"isrc"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type genreResponse struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type pagination struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type listResponse[T any] struct {
	Data       []T         `json:"data"`
	Pagination *pagination `json:"pagination,omitempty"`
}

func toArtist(a domain.Artist) artistResponse {
	return artistResponse{ID: a.ID.String(), Name: a.Name, CreatedAt: a.CreatedAt.UTC(), UpdatedAt: a.UpdatedAt.UTC()}
}

func toAlbum(a domain.Album) albumResponse {
	return albumResponse{
		ID:          a.ID.String(),
		Title:       a.Title,
		AlbumType:   string(a.Type),
		ReleaseDate: a.ReleaseDate.Format(time.DateOnly),
		ArtistIDs:   idStrings(a.ArtistIDs),
		GenreIDs:    idStrings(a.GenreIDs),
		CreatedAt:   a.CreatedAt.UTC(),
		UpdatedAt:   a.UpdatedAt.UTC(),
	}
}

func toTrack(t domain.Track) trackResponse {
	var isrc *string
	if t.ISRC != "" {
		v := t.ISRC
		isrc = &v
	}
	return trackResponse{
		ID:          t.ID.String(),
		AlbumID:     t.AlbumID.String(),
		ArtistIDs:   idStrings(t.ArtistIDs),
		Title:       t.Title,
		DurationMs:  t.Duration.Milliseconds(),
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
		Explicit:    t.Explicit,
		ISRC:        isrc,
		Status:      string(t.Status),
		CreatedAt:   t.CreatedAt.UTC(),
		UpdatedAt:   t.UpdatedAt.UTC(),
	}
}

func toGenre(g domain.Genre) genreResponse {
	return genreResponse{ID: g.ID.String(), Slug: g.Slug, Name: g.Name}
}

func mapSlice[S, D any](in []S, f func(S) D) []D {
	out := make([]D, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
