// Package catalog reads Catalog Service over its REST API.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

// Client implements application.Catalog.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client for Catalog at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), hc: hc}
}

type albumDTO struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	AlbumType   string    `json:"albumType"`
	ReleaseDate string    `json:"releaseDate"`
	ArtistIDs   []string  `json:"artistIds"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (a albumDTO) album() application.Album {
	return application.Album{ID: a.ID, Title: a.Title, AlbumType: a.AlbumType, ReleaseDate: a.ReleaseDate, ArtistIDs: a.ArtistIDs, UpdatedAt: a.UpdatedAt}
}

type artistDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type trackDTO struct {
	ID         string    `json:"id"`
	AlbumID    string    `json:"albumId"`
	ArtistIDs  []string  `json:"artistIds"`
	Title      string    `json:"title"`
	DurationMs int64     `json:"durationMs"`
	Explicit   bool      `json:"explicit"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return application.ErrNotFound
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("catalog: GET %s: status %d", path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("catalog: decode %s: %w", path, err)
	}
	return nil
}

// Album reads one album.
func (c *Client) Album(ctx context.Context, id string) (application.Album, error) {
	var a albumDTO
	if err := c.get(ctx, "/api/v1/albums/"+url.PathEscape(id), &a); err != nil {
		return application.Album{}, err
	}
	return a.album(), nil
}

// maxIDs is Catalog's limit for GET /artists?ids=.
const maxIDs = 100

// Artists reads artists by ID (unknown IDs are absent from the map).
func (c *Client) Artists(ctx context.Context, ids []string) (map[string]application.Artist, error) {
	out := map[string]application.Artist{}
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	for chunk := range slices.Chunk(ids, maxIDs) {
		var res struct {
			Data []artistDTO `json:"data"`
		}
		if err := c.get(ctx, "/api/v1/artists?ids="+url.QueryEscape(strings.Join(chunk, ",")), &res); err != nil {
			return nil, err
		}
		for _, a := range res.Data {
			out[a.ID] = application.Artist{ID: a.ID, Name: a.Name, UpdatedAt: a.UpdatedAt}
		}
	}
	return out, nil
}

// EachAlbum walks every album page by page (cursor pagination).
func (c *Client) EachAlbum(ctx context.Context, fn func(application.Album) error) error {
	cursor := ""
	for {
		path := "/api/v1/albums?limit=100"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		var page struct {
			Data       []albumDTO `json:"data"`
			Pagination struct {
				NextCursor string `json:"nextCursor"`
				HasMore    bool   `json:"hasMore"`
			} `json:"pagination"`
		}
		if err := c.get(ctx, path, &page); err != nil {
			return err
		}
		for _, a := range page.Data {
			if err := fn(a.album()); err != nil {
				return err
			}
		}
		if !page.Pagination.HasMore || page.Pagination.NextCursor == "" {
			return nil
		}
		cursor = page.Pagination.NextCursor
	}
}

// AlbumTracks reads an album's tracks.
func (c *Client) AlbumTracks(ctx context.Context, albumID string) ([]application.Track, error) {
	var res struct {
		Data []trackDTO `json:"data"`
	}
	if err := c.get(ctx, "/api/v1/albums/"+url.PathEscape(albumID)+"/tracks", &res); err != nil {
		return nil, err
	}
	out := make([]application.Track, len(res.Data))
	for i, t := range res.Data {
		out[i] = application.Track{ID: t.ID, Title: t.Title, AlbumID: t.AlbumID, ArtistIDs: t.ArtistIDs, DurationMs: t.DurationMs, Explicit: t.Explicit, Status: t.Status, UpdatedAt: t.UpdatedAt}
	}
	return out, nil
}
