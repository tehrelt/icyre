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

// MediaIngestClient calls Media Ingest's internal moderation endpoints.
type MediaIngestClient struct {
	base  string
	token string
	hc    *http.Client
}

// NewMediaIngestClient returns a MediaIngestClient.
func NewMediaIngestClient(baseURL, internalToken string, hc *http.Client) *MediaIngestClient {
	return &MediaIngestClient{
		base:  strings.TrimRight(baseURL, "/"),
		token: internalToken,
		hc:    hc,
	}
}

type ingestUploadDTO struct {
	ID            string     `json:"id"`
	TrackID       string     `json:"trackId"`
	Status        string     `json:"status"`
	FailureReason *string    `json:"failureReason"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	CompletedAt   *time.Time `json:"completedAt"`
}

type ingestUploadsResponse struct {
	Data       []ingestUploadDTO `json:"data"`
	Pagination paginationDTO     `json:"pagination"`
}

// ListUploads fetches a page of upload sessions from Media Ingest.
func (c *MediaIngestClient) ListUploads(ctx context.Context, q domain.IngestListQuery) (domain.Page[domain.IngestUpload], error) {
	params := url.Values{}
	if q.TrackID != nil {
		params.Set("trackId", q.TrackID.String())
	}
	if q.Status != "" {
		params.Set("status", q.Status)
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}
	endpoint := c.base + "/internal/v1/moderation/uploads"
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	var res ingestUploadsResponse
	if err := doJSON(ctx, c.hc, c.token, http.MethodGet, endpoint, nil, &res); err != nil {
		return domain.Page[domain.IngestUpload]{}, err
	}
	items := make([]domain.IngestUpload, 0, len(res.Data))
	for _, dto := range res.Data {
		id, err := uuid.Parse(dto.ID)
		if err != nil {
			return domain.Page[domain.IngestUpload]{}, fmt.Errorf("%w: invalid upload id", domain.ErrUpstreamUnavailable)
		}
		trackID, err := uuid.Parse(dto.TrackID)
		if err != nil {
			return domain.Page[domain.IngestUpload]{}, fmt.Errorf("%w: invalid track id", domain.ErrUpstreamUnavailable)
		}
		items = append(items, domain.IngestUpload{
			ID:            id,
			TrackID:       trackID,
			Status:        dto.Status,
			FailureReason: dto.FailureReason,
			CreatedAt:     dto.CreatedAt,
			ExpiresAt:     dto.ExpiresAt,
			CompletedAt:   dto.CompletedAt,
		})
	}
	return domain.Page[domain.IngestUpload]{
		Items:      items,
		NextCursor: res.Pagination.NextCursor,
		HasMore:    res.Pagination.HasMore,
	}, nil
}
