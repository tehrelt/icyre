// Package library reads the listener's saved tracks from Library Service
// with the listener's own access token.
package library

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

// Client implements ports.Library.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client for Library at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), hc: hc}
}

// maxIDs is Library's limit for one contains lookup.
const maxIDs = 100

// SavedTracks returns the saved subset of ids. Anonymous requests (no
// token) and rejected tokens simply have no saved tracks.
func (c *Client) SavedTracks(ctx context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	token := ports.UserToken(ctx)
	if token == "" || len(ids) == 0 {
		return out, nil
	}
	for chunk := range slices.Chunk(ids, maxIDs) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			c.base+"/api/v1/me/library/tracks/contains?ids="+url.QueryEscape(strings.Join(chunk, ",")), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		res, err := c.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("library: %w", err)
		}
		var body struct {
			Data []string `json:"data"`
		}
		switch res.StatusCode {
		case http.StatusOK:
			err = json.NewDecoder(res.Body).Decode(&body)
		case http.StatusUnauthorized:
			// Expired or revoked token: the client refreshes on its own calls.
		default:
			err = fmt.Errorf("library: status %d", res.StatusCode)
		}
		_ = res.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, id := range body.Data {
			out[id] = true
		}
	}
	return out, nil
}
