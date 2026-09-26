// Package catalog reads track albums and artists from Catalog Service.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/workers/analytics/internal/domain"
)

// maxIDs is Catalog's limit for GET /tracks?ids=.
const maxIDs = 100

// Client implements application.Catalog.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client for Catalog at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), hc: hc}
}

type trackDTO struct {
	ID        string   `json:"id"`
	AlbumID   string   `json:"albumId"`
	ArtistIDs []string `json:"artistIds"`
}

// Tracks reads tracks by ID; unknown IDs are absent from the map.
func (c *Client) Tracks(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.TrackInfo, error) {
	out := make(map[uuid.UUID]domain.TrackInfo, len(ids))
	for chunk := range slices.Chunk(ids, maxIDs) {
		strs := make([]string, len(chunk))
		for i, id := range chunk {
			strs[i] = id.String()
		}
		var res struct {
			Data []trackDTO `json:"data"`
		}
		if err := c.get(ctx, "/api/v1/tracks?ids="+url.QueryEscape(strings.Join(strs, ",")), &res); err != nil {
			return nil, err
		}
		for _, t := range res.Data {
			id, err := uuid.Parse(t.ID)
			if err != nil {
				return nil, fmt.Errorf("catalog: track id %q: %w", t.ID, err)
			}
			info := domain.TrackInfo{}
			if a, err := uuid.Parse(t.AlbumID); err == nil {
				info.AlbumID = a
			}
			for _, s := range t.ArtistIDs {
				if a, err := uuid.Parse(s); err == nil {
					info.ArtistIDs = append(info.ArtistIDs, a)
				}
			}
			out[id] = info
		}
	}
	return out, nil
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
