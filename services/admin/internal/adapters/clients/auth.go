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

// AuthClient calls Auth's internal moderation endpoints.
type AuthClient struct {
	base  string
	token string
	hc    *http.Client
}

// NewAuthClient returns an AuthClient.
func NewAuthClient(baseURL, internalToken string, hc *http.Client) *AuthClient {
	return &AuthClient{
		base:  strings.TrimRight(baseURL, "/"),
		token: internalToken,
		hc:    hc,
	}
}

type authUserDTO struct {
	ID        string     `json:"id"`
	Roles     []string   `json:"roles"`
	Blocked   bool       `json:"blocked"`
	BlockedAt *time.Time `json:"blockedAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type authUsersResponse struct {
	Data       []authUserDTO `json:"data"`
	Pagination paginationDTO `json:"pagination"`
}

type moderateUserRequest struct {
	OperationID string `json:"operationId"`
	Blocked     bool   `json:"blocked"`
}

type authModerationReceiptDTO struct {
	OperationID        string    `json:"operationId"`
	UserID             string    `json:"userId"`
	Blocked            bool      `json:"blocked"`
	Changed            bool      `json:"changed"`
	RevokedSessions    int       `json:"revokedSessions"`
	RevocationsPending bool      `json:"revocationsPending"`
	CreatedAt          time.Time `json:"createdAt"`
}

// ListUsers fetches a page of users from Auth.
func (c *AuthClient) ListUsers(ctx context.Context, q domain.UserListQuery) (domain.Page[domain.UserSummary], error) {
	params := url.Values{}
	if q.Blocked != nil {
		params.Set("blocked", strconv.FormatBool(*q.Blocked))
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}
	endpoint := c.base + "/internal/v1/moderation/users"
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	var res authUsersResponse
	if err := doJSON(ctx, c.hc, c.token, http.MethodGet, endpoint, nil, &res); err != nil {
		return domain.Page[domain.UserSummary]{}, err
	}
	items := make([]domain.UserSummary, 0, len(res.Data))
	for _, dto := range res.Data {
		id, err := uuid.Parse(dto.ID)
		if err != nil {
			return domain.Page[domain.UserSummary]{}, fmt.Errorf("%w: invalid user id", domain.ErrUpstreamUnavailable)
		}
		roles := dto.Roles
		if roles == nil {
			roles = []string{}
		}
		items = append(items, domain.UserSummary{
			ID:        id,
			Roles:     roles,
			Blocked:   dto.Blocked,
			BlockedAt: dto.BlockedAt,
			CreatedAt: dto.CreatedAt,
			UpdatedAt: dto.UpdatedAt,
		})
	}
	return domain.Page[domain.UserSummary]{
		Items:      items,
		NextCursor: res.Pagination.NextCursor,
		HasMore:    res.Pagination.HasMore,
	}, nil
}

// ModerateUser sends a block/unblock command to Auth.
func (c *AuthClient) ModerateUser(ctx context.Context, operationID, userID uuid.UUID, blocked bool) (domain.UserModerationReceipt, error) {
	endpoint := c.base + "/internal/v1/moderation/users/" + userID.String()
	req := moderateUserRequest{OperationID: operationID.String(), Blocked: blocked}
	var res authModerationReceiptDTO
	if err := doJSON(ctx, c.hc, c.token, http.MethodPatch, endpoint, req, &res); err != nil {
		return domain.UserModerationReceipt{}, err
	}
	opID, err := uuid.Parse(res.OperationID)
	if err != nil {
		return domain.UserModerationReceipt{}, fmt.Errorf("%w: invalid operationId", domain.ErrUpstreamUnavailable)
	}
	uID, err := uuid.Parse(res.UserID)
	if err != nil {
		return domain.UserModerationReceipt{}, fmt.Errorf("%w: invalid userId", domain.ErrUpstreamUnavailable)
	}
	return domain.UserModerationReceipt{
		OperationID:        opID,
		UserID:             uID,
		Blocked:            res.Blocked,
		Changed:            res.Changed,
		RevokedSessions:    res.RevokedSessions,
		RevocationsPending: res.RevocationsPending,
		CreatedAt:          res.CreatedAt,
	}, nil
}
