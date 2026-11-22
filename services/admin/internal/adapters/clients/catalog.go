package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

// CatalogClient calls Catalog's internal moderation endpoints.
type CatalogClient struct {
	base  string
	token string
	hc    *http.Client
}

// NewCatalogClient returns a CatalogClient.
func NewCatalogClient(baseURL, internalToken string, hc *http.Client) *CatalogClient {
	return &CatalogClient{
		base:  strings.TrimRight(baseURL, "/"),
		token: internalToken,
		hc:    hc,
	}
}

type catalogTrackDTO struct {
	ID          string    `json:"id"`
	AlbumID     string    `json:"albumId"`
	ArtistIDs   []string  `json:"artistIds"`
	Title       string    `json:"title"`
	DurationMs  int64     `json:"durationMs"`
	TrackNumber int       `json:"trackNumber"`
	DiscNumber  int       `json:"discNumber"`
	Explicit    bool      `json:"explicit"`
	ISRC        *string   `json:"isrc"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type catalogTracksResponse struct {
	Data       []catalogTrackDTO `json:"data"`
	Pagination paginationDTO     `json:"pagination"`
}

type moderateTrackRequest struct {
	OperationID string `json:"operationId"`
	Blocked     bool   `json:"blocked"`
}

type catalogModerationReceiptDTO struct {
	OperationID    string    `json:"operationId"`
	TrackID        string    `json:"trackId"`
	PreviousStatus string    `json:"previousStatus"`
	CurrentStatus  string    `json:"currentStatus"`
	Changed        bool      `json:"changed"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ListTracks fetches a page of tracks from Catalog.
func (c *CatalogClient) ListTracks(ctx context.Context, q domain.TrackListQuery) (domain.Page[domain.TrackSummary], error) {
	params := url.Values{}
	if q.Status != "" {
		params.Set("status", q.Status)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}
	endpoint := c.base + "/internal/v1/moderation/tracks"
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	var res catalogTracksResponse
	if err := doJSON(ctx, c.hc, c.token, http.MethodGet, endpoint, nil, &res); err != nil {
		return domain.Page[domain.TrackSummary]{}, err
	}
	items := make([]domain.TrackSummary, 0, len(res.Data))
	for _, dto := range res.Data {
		item, err := toTrackSummary(dto)
		if err != nil {
			return domain.Page[domain.TrackSummary]{}, fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
		}
		items = append(items, item)
	}
	return domain.Page[domain.TrackSummary]{
		Items:      items,
		NextCursor: res.Pagination.NextCursor,
		HasMore:    res.Pagination.HasMore,
	}, nil
}

// ModerateTrack sends a block/unblock command to Catalog.
func (c *CatalogClient) ModerateTrack(ctx context.Context, operationID, trackID uuid.UUID, blocked bool) (domain.TrackModerationReceipt, error) {
	endpoint := c.base + "/internal/v1/moderation/tracks/" + trackID.String()
	req := moderateTrackRequest{OperationID: operationID.String(), Blocked: blocked}
	var res catalogModerationReceiptDTO
	if err := doJSON(ctx, c.hc, c.token, http.MethodPatch, endpoint, req, &res); err != nil {
		return domain.TrackModerationReceipt{}, err
	}
	opID, err := uuid.Parse(res.OperationID)
	if err != nil {
		return domain.TrackModerationReceipt{}, fmt.Errorf("%w: invalid operationId", domain.ErrUpstreamUnavailable)
	}
	tID, err := uuid.Parse(res.TrackID)
	if err != nil {
		return domain.TrackModerationReceipt{}, fmt.Errorf("%w: invalid trackId", domain.ErrUpstreamUnavailable)
	}
	return domain.TrackModerationReceipt{
		OperationID:    opID,
		TrackID:        tID,
		PreviousStatus: res.PreviousStatus,
		CurrentStatus:  res.CurrentStatus,
		Changed:        res.Changed,
		CreatedAt:      res.CreatedAt,
	}, nil
}

func toTrackSummary(dto catalogTrackDTO) (domain.TrackSummary, error) {
	id, err := uuid.Parse(dto.ID)
	if err != nil {
		return domain.TrackSummary{}, fmt.Errorf("invalid track id: %w", err)
	}
	albumID, err := uuid.Parse(dto.AlbumID)
	if err != nil {
		return domain.TrackSummary{}, fmt.Errorf("invalid album id: %w", err)
	}
	artists := make([]uuid.UUID, 0, len(dto.ArtistIDs))
	for _, raw := range dto.ArtistIDs {
		aid, err := uuid.Parse(raw)
		if err != nil {
			return domain.TrackSummary{}, fmt.Errorf("invalid artist id: %w", err)
		}
		artists = append(artists, aid)
	}
	return domain.TrackSummary{
		ID:          id,
		AlbumID:     albumID,
		ArtistIDs:   artists,
		Title:       dto.Title,
		DurationMs:  dto.DurationMs,
		TrackNumber: dto.TrackNumber,
		DiscNumber:  dto.DiscNumber,
		Explicit:    dto.Explicit,
		ISRC:        dto.ISRC,
		Status:      dto.Status,
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
	}, nil
}
