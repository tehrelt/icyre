// Package catalog reads the recommendable tracks from Catalog Service.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/workers/recommendation/internal/domain"
)

// statusReady is the only playable track status.
const statusReady = "READY"

// Client implements application.CatalogSource.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client for Catalog at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), hc: hc}
}

type albumDTO struct {
	ID          string   `json:"id"`
	ReleaseDate string   `json:"releaseDate"`
	GenreIDs    []string `json:"genreIds"`
}

type trackDTO struct {
	ID        string   `json:"id"`
	ArtistIDs []string `json:"artistIds"`
	Status    string   `json:"status"`
}

// Tracks walks every album (cursor pagination) and returns its playable
// tracks with the album's genres and release date. One request per album:
// fine for the MVP catalog; a catalog.events-fed copy replaces it at scale.
func (c *Client) Tracks(ctx context.Context) ([]domain.Track, error) {
	var out []domain.Track
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
			return nil, err
		}
		for _, a := range page.Data {
			tracks, err := c.albumTracks(ctx, a)
			if err != nil {
				return nil, err
			}
			out = append(out, tracks...)
		}
		if !page.Pagination.HasMore || page.Pagination.NextCursor == "" {
			return out, nil
		}
		cursor = page.Pagination.NextCursor
	}
}

func (c *Client) albumTracks(ctx context.Context, a albumDTO) ([]domain.Track, error) {
	album, err := uuid.Parse(a.ID)
	if err != nil {
		return nil, fmt.Errorf("catalog: album id %q: %w", a.ID, err)
	}
	released, _ := time.Parse(time.DateOnly, a.ReleaseDate) // unknown → zero
	genres := parseIDs(a.GenreIDs)
	var res struct {
		Data []trackDTO `json:"data"`
	}
	if err := c.get(ctx, "/api/v1/albums/"+url.PathEscape(a.ID)+"/tracks", &res); err != nil {
		return nil, err
	}
	var out []domain.Track
	for _, t := range res.Data {
		id, err := uuid.Parse(t.ID)
		if err != nil || t.Status != statusReady {
			continue
		}
		out = append(out, domain.Track{ID: id, ArtistIDs: parseIDs(t.ArtistIDs), AlbumID: album, GenreIDs: genres, Released: released})
	}
	return out, nil
}

func parseIDs(raw []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
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
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog: GET %s: status %d", path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("catalog: decode %s: %w", path, err)
	}
	return nil
}
