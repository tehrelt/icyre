// Package catalog reads track status from Catalog Service over HTTP.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/tehrelt/icyre/services/stream-auth/internal/domain"
)

// Client implements application.Tracks.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client for Catalog at baseURL (e.g. http://catalog:8080).
func New(baseURL string, hc *http.Client) *Client { return &Client{base: baseURL, hc: hc} }

// Status returns the track's lifecycle status.
func (c *Client) Status(ctx context.Context, trackID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/tracks/"+url.PathEscape(trackID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("catalog: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", domain.ErrTrackNotFound
	default:
		return "", fmt.Errorf("catalog: status %d", res.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("catalog: decode: %w", err)
	}
	return body.Status, nil
}

// Tracks is the uncached status lookup.
type Tracks interface {
	Status(ctx context.Context, trackID string) (string, error)
}

// StatusCache stores statuses briefly (see CachedTracks).
type StatusCache interface {
	GetOrLoad(ctx context.Context, id string, load func(context.Context) (string, error)) (string, error)
}

// CachedTracks keeps statuses for a few seconds, so a listener skipping
// through an album does not hit Catalog on every tap. A block takes effect
// within the cache TTL; URLs already issued live until they expire.
type CachedTracks struct {
	next  Tracks
	cache StatusCache
}

// NewCachedTracks wraps next.
func NewCachedTracks(next Tracks, cache StatusCache) *CachedTracks {
	return &CachedTracks{next: next, cache: cache}
}

// Status implements application.Tracks.
func (c *CachedTracks) Status(ctx context.Context, trackID string) (string, error) {
	return c.cache.GetOrLoad(ctx, trackID, func(ctx context.Context) (string, error) { return c.next.Status(ctx, trackID) })
}
