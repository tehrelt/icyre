// Package catalog checks that tracks and albums exist in Catalog.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/library/internal/domain"
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

// Exists reports domain.ErrNotFound for unknown or deleted items.
func (c *Client) Exists(ctx context.Context, kind domain.Kind, id uuid.UUID) error {
	path := "/api/v1/tracks/"
	if kind == domain.KindAlbum {
		path = "/api/v1/albums/"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+id.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return domain.ErrNotFound
	default:
		return fmt.Errorf("catalog: status %d", res.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return fmt.Errorf("catalog: decode: %w", err)
	}
	if body.Status == "DELETED" {
		return domain.ErrNotFound
	}
	return nil
}
