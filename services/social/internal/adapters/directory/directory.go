// Package directory checks follow targets: users in User Profile, artists in Catalog.
package directory

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/tehrelt/icyre/services/social/internal/domain"
)

// Client implements application.Directory.
type Client struct {
	profile string
	catalog string
	hc      *http.Client
}

// New returns a Client for User Profile at profileURL and Catalog at catalogURL.
func New(profileURL, catalogURL string, hc *http.Client) *Client {
	return &Client{profile: strings.TrimRight(profileURL, "/"), catalog: strings.TrimRight(catalogURL, "/"), hc: hc}
}

// Exists reports domain.ErrNotFound for unknown users and artists.
func (c *Client) Exists(ctx context.Context, t domain.Target) error {
	url := c.profile + "/api/v1/users/" + t.ID.String()
	if t.Type == domain.TargetArtist {
		url = c.catalog + "/api/v1/artists/" + t.ID.String()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s lookup: %w", t.Type, err)
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return domain.ErrNotFound
	default:
		return fmt.Errorf("%s lookup: status %d", t.Type, res.StatusCode)
	}
}
