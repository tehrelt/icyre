// Package history reads the listener's recently played sources from the
// Listening History Service with the listener's own access token.
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

// Client implements ports.History.
type Client struct {
	base string
	hc   *http.Client
}

// New returns a Client for Listening History at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), hc: hc}
}

// RecentSources returns recently played sources; anonymous or rejected
// tokens have no history.
func (c *Client) RecentSources(ctx context.Context, limit int) ([]string, error) {
	token := ports.UserToken(ctx)
	if token == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/me/history/sources?limit="+strconv.Itoa(limit), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, nil
	default:
		return nil, fmt.Errorf("history: status %d", res.StatusCode)
	}
	var body struct {
		Data []struct {
			Source string `json:"source"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("history: decode: %w", err)
	}
	out := make([]string, len(body.Data))
	for i, s := range body.Data {
		out[i] = s.Source
	}
	return out, nil
}
