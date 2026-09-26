// Package playlist reads the Playlist Service and public User Profiles.
package playlist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

type upstream struct {
	name string
	base string
	hc   *http.Client
}

// get performs a GET and decodes JSON; 404 maps to ports.ErrNotFound.
func (u upstream) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := u.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", u.name, err)
	}
	defer func() { _ = res.Body.Close() }()
	switch {
	case res.StatusCode == http.StatusNotFound:
		_, _ = io.Copy(io.Discard, res.Body)
		return fmt.Errorf("%s: %w", u.name, ports.ErrNotFound)
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("%s: GET %s: status %d", u.name, path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		return fmt.Errorf("%s: decode: %w", u.name, err)
	}
	return nil
}

// Client implements ports.Playlists.
type Client struct{ u upstream }

// New returns a Client for the Playlist Service at baseURL.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{u: upstream{name: "playlist", base: strings.TrimRight(baseURL, "/"), hc: hc}}
}

// GetPlaylist implements ports.Playlists.
func (c *Client) GetPlaylist(ctx context.Context, id string) (ports.Playlist, error) {
	var dto struct {
		ID        string    `json:"id"`
		OwnerID   string    `json:"ownerId"`
		Title     string    `json:"title"`
		UpdatedAt time.Time `json:"updatedAt"`
		Tracks    []struct {
			TrackID string `json:"trackId"`
		} `json:"tracks"`
	}
	if err := c.u.get(ctx, "/api/v1/playlists/"+url.PathEscape(id), &dto); err != nil {
		return ports.Playlist{}, err
	}
	p := ports.Playlist{ID: dto.ID, OwnerID: dto.OwnerID, Title: dto.Title, UpdatedAt: dto.UpdatedAt, TrackIDs: make([]string, len(dto.Tracks))}
	for i, t := range dto.Tracks {
		p.TrackIDs[i] = t.TrackID
	}
	return p, nil
}

// Profiles implements ports.Profiles.
type Profiles struct{ u upstream }

// NewProfiles returns Profiles for User Profile at baseURL.
func NewProfiles(baseURL string, hc *http.Client) *Profiles {
	return &Profiles{u: upstream{name: "user-profile", base: strings.TrimRight(baseURL, "/"), hc: hc}}
}

// DisplayName implements ports.Profiles: the display name, else the username.
func (p *Profiles) DisplayName(ctx context.Context, userID string) (string, error) {
	var dto struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	}
	if err := p.u.get(ctx, "/api/v1/users/"+url.PathEscape(userID), &dto); err != nil {
		return "", err
	}
	if dto.DisplayName != "" {
		return dto.DisplayName, nil
	}
	return dto.Username, nil
}
