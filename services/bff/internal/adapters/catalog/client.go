// Package catalog is the HTTP adapter to Catalog Service's REST API.
//
// Internal traffic will move to gRPC with EPIC-034; the port stays the same.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tehrelt/icyre/services/bff/internal/ports"
)

// Client implements ports.Catalog.
type Client struct {
	base    string
	http    *http.Client
	metrics *prometheus.HistogramVec
}

// New returns a client for baseURL (e.g. http://catalog:8080). reg may be nil.
func New(baseURL string, hc *http.Client, reg prometheus.Registerer) *Client {
	c := &Client{base: strings.TrimRight(baseURL, "/"), http: hc}
	if reg != nil {
		c.metrics = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bff_upstream_request_duration_seconds",
			Help:    "Upstream calls made by the BFF, by upstream, operation and result.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"upstream", "operation", "result"})
		reg.MustRegister(c.metrics)
	}
	return c
}

// Wire formats of Catalog Service (its public REST contract).
type albumDTO struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	AlbumType   string   `json:"albumType"`
	ReleaseDate string   `json:"releaseDate"`
	ArtistIDs   []string `json:"artistIds"`
	GenreIDs    []string `json:"genreIds"`
}

type trackDTO struct {
	ID          string   `json:"id"`
	AlbumID     string   `json:"albumId"`
	ArtistIDs   []string `json:"artistIds"`
	Title       string   `json:"title"`
	DurationMs  int64    `json:"durationMs"`
	TrackNumber int      `json:"trackNumber"`
	DiscNumber  int      `json:"discNumber"`
	Explicit    bool     `json:"explicit"`
	Status      string   `json:"status"`
}

type artistDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type genreDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type list[T any] struct {
	Data []T `json:"data"`
}

// GetAlbum implements ports.Catalog.
func (c *Client) GetAlbum(ctx context.Context, id string) (ports.Album, error) {
	var dto albumDTO
	if err := c.get(ctx, "get_album", "/api/v1/albums/"+url.PathEscape(id), &dto); err != nil {
		return ports.Album{}, err
	}
	return toAlbum(dto)
}

// AlbumTracks implements ports.Catalog.
func (c *Client) AlbumTracks(ctx context.Context, albumID string) ([]ports.Track, error) {
	var res list[trackDTO]
	if err := c.get(ctx, "album_tracks", "/api/v1/albums/"+url.PathEscape(albumID)+"/tracks", &res); err != nil {
		return nil, err
	}
	return toTracks(res.Data), nil
}

// maxBatch is Catalog's limit for ?ids= lookups.
const maxBatch = 100

// Tracks implements ports.Catalog, in batches of maxBatch.
func (c *Client) Tracks(ctx context.Context, ids []string) ([]ports.Track, error) {
	out := make([]ports.Track, 0, len(ids))
	for chunk := range slices.Chunk(ids, maxBatch) {
		var res list[trackDTO]
		q := url.Values{"ids": {strings.Join(chunk, ",")}}
		if err := c.get(ctx, "tracks_batch", "/api/v1/tracks?"+q.Encode(), &res); err != nil {
			return nil, err
		}
		out = append(out, toTracks(res.Data)...)
	}
	return out, nil
}

func toTracks(dtos []trackDTO) []ports.Track {
	out := make([]ports.Track, 0, len(dtos))
	for _, t := range dtos {
		out = append(out, ports.Track{
			ID: t.ID, AlbumID: t.AlbumID, ArtistIDs: t.ArtistIDs, Title: t.Title,
			Duration: time.Duration(t.DurationMs) * time.Millisecond, TrackNumber: t.TrackNumber,
			DiscNumber: t.DiscNumber, Explicit: t.Explicit, Status: t.Status,
		})
	}
	return out
}

// LatestAlbums implements ports.Catalog.
func (c *Client) LatestAlbums(ctx context.Context, limit int) ([]ports.Album, error) {
	return c.albums(ctx, "latest_albums", "/api/v1/albums?limit="+strconv.Itoa(limit))
}

// ArtistAlbums implements ports.Catalog.
func (c *Client) ArtistAlbums(ctx context.Context, artistID string, limit int) ([]ports.Album, error) {
	return c.albums(ctx, "artist_albums", "/api/v1/artists/"+url.PathEscape(artistID)+"/albums?limit="+strconv.Itoa(limit))
}

// Artists implements ports.Catalog.
func (c *Client) Artists(ctx context.Context, ids []string) ([]ports.Artist, error) {
	var res list[artistDTO]
	q := url.Values{"ids": {strings.Join(ids, ",")}}
	if err := c.get(ctx, "artists_batch", "/api/v1/artists?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	out := make([]ports.Artist, 0, len(res.Data))
	for _, a := range res.Data {
		out = append(out, ports.Artist(a))
	}
	return out, nil
}

// Genres implements ports.Catalog.
func (c *Client) Genres(ctx context.Context) ([]ports.Genre, error) {
	var res list[genreDTO]
	if err := c.get(ctx, "genres", "/api/v1/genres", &res); err != nil {
		return nil, err
	}
	out := make([]ports.Genre, 0, len(res.Data))
	for _, g := range res.Data {
		out = append(out, ports.Genre(g))
	}
	return out, nil
}

func (c *Client) albums(ctx context.Context, op, path string) ([]ports.Album, error) {
	var res list[albumDTO]
	if err := c.get(ctx, op, path, &res); err != nil {
		return nil, err
	}
	out := make([]ports.Album, 0, len(res.Data))
	for _, dto := range res.Data {
		a, err := toAlbum(dto)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func toAlbum(dto albumDTO) (ports.Album, error) {
	d, err := time.Parse(time.DateOnly, dto.ReleaseDate)
	if err != nil {
		return ports.Album{}, fmt.Errorf("catalog album %s: bad releaseDate %q", dto.ID, dto.ReleaseDate)
	}
	return ports.Album{ID: dto.ID, Title: dto.Title, AlbumType: dto.AlbumType, ReleaseDate: d, ArtistIDs: dto.ArtistIDs, GenreIDs: dto.GenreIDs}, nil
}

// get performs a GET and decodes JSON; 404 maps to ports.ErrNotFound.
func (c *Client) get(ctx context.Context, op, path string, dst any) (err error) {
	start := time.Now()
	defer func() {
		if c.metrics == nil {
			return
		}
		result := "ok"
		switch {
		case errors.Is(err, ports.ErrNotFound):
			result = "not_found"
		case err != nil:
			result = "error"
		}
		c.metrics.WithLabelValues("catalog", op, result).Observe(time.Since(start).Seconds())
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("catalog %s: %w", op, err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("catalog %s: %w", op, err)
	}
	defer func() { _ = res.Body.Close() }()

	switch {
	case res.StatusCode == http.StatusNotFound:
		_, _ = io.Copy(io.Discard, res.Body)
		return fmt.Errorf("catalog %s: %w", op, ports.ErrNotFound)
	case res.StatusCode >= 400:
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return fmt.Errorf("catalog %s: status %d", op, res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(dst); err != nil {
		return fmt.Errorf("catalog %s: decode: %w", op, err)
	}
	return nil
}
