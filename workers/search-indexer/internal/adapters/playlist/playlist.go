// Package playlist reads the Playlist Service and User Profile over REST.
package playlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tehrelt/icyre/workers/search-indexer/internal/application"
)

type client struct {
	name string
	base string
	hc   *http.Client
}

func (c client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", c.name, err)
	}
	defer func() { _ = res.Body.Close() }()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return application.ErrNotFound
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("%s: GET %s: status %d", c.name, path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: decode %s: %w", c.name, path, err)
	}
	return nil
}

// Client implements application.Playlists.
type Client struct{ c client }

// New returns a Client for the Playlist Service at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{c: client{name: "playlist", base: strings.TrimRight(baseURL, "/"), hc: hc}}
}

type playlistDTO struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"ownerId"`
	Title      string    `json:"title"`
	TrackCount int       `json:"trackCount"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (p playlistDTO) playlist() application.Playlist {
	return application.Playlist{ID: p.ID, OwnerID: p.OwnerID, Title: p.Title, TrackCount: p.TrackCount, UpdatedAt: p.UpdatedAt}
}

// Playlist reads one playlist.
func (c *Client) Playlist(ctx context.Context, id string) (application.Playlist, error) {
	var p playlistDTO
	if err := c.c.get(ctx, "/api/v1/playlists/"+url.PathEscape(id), &p); err != nil {
		return application.Playlist{}, err
	}
	return p.playlist(), nil
}

// EachPlaylist walks every playlist through the internal listing.
func (c *Client) EachPlaylist(ctx context.Context, fn func(application.Playlist) error) error {
	after := ""
	for {
		var page struct {
			Data      []playlistDTO `json:"data"`
			NextAfter string        `json:"nextAfter"`
		}
		if err := c.c.get(ctx, "/internal/v1/playlists?limit=500&after="+url.QueryEscape(after), &page); err != nil {
			return err
		}
		for _, p := range page.Data {
			if err := fn(p.playlist()); err != nil {
				return err
			}
		}
		if len(page.Data) == 0 || page.NextAfter == "" {
			return nil
		}
		after = page.NextAfter
	}
}

// Profiles implements application.Profiles.
type Profiles struct{ c client }

// NewProfiles returns Profiles for User Profile at baseURL.
func NewProfiles(baseURL string, hc *http.Client) *Profiles {
	return &Profiles{c: client{name: "user-profile", base: strings.TrimRight(baseURL, "/"), hc: hc}}
}

// DisplayNames reads each user's public profile; unknown users are absent.
func (p *Profiles) DisplayNames(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		var u struct {
			DisplayName string `json:"displayName"`
			Username    string `json:"username"`
		}
		err := p.c.get(ctx, "/api/v1/users/"+url.PathEscape(id), &u)
		if errors.Is(err, application.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = u.DisplayName
		if out[id] == "" {
			out[id] = u.Username
		}
	}
	return out, nil
}
