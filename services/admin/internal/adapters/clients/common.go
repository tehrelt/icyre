// Package clients implements HTTP clients for internal moderation APIs.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

const maxErrorBodyBytes = 16 << 10

type paginationDTO struct {
	NextCursor *string `json:"nextCursor"`
	HasMore    bool    `json:"hasMore"`
}

type errorEnvelope struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

var safeOwnerMessages = map[string]string{
	"TRACK_NOT_FOUND":               "Track not found",
	"ACCOUNT_NOT_FOUND":             "Account not found",
	"INVALID_MODERATION_TRANSITION": "Track cannot be moderated in its current status",
	"MODERATION_CONFLICT":           "Moderation operation conflicts with an existing receipt",
	"OPERATION_CONFLICT":            "Operation ID already used with different parameters",
	"CANNOT_BLOCK_ADMIN":            "Cannot block an admin account",
	"TRACK_DELETED":                 "Deleted track cannot be modified",
	"INVALID_CURSOR":                "Cursor is malformed",
	"VALIDATION_FAILED":             "Request validation failed",
	"BAD_REQUEST":                   "Invalid request",
}

func doJSON(ctx context.Context, hc *http.Client, token, method, url string, reqBody, dst any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(authn.InternalTokenHeader, token)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrUpstreamUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		if dst == nil {
			return nil
		}
		if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
			return fmt.Errorf("%w: decode response: %v", domain.ErrUpstreamUnavailable, err)
		}
		return nil
	}
	return mapUpstreamError(res)
}

func mapUpstreamError(res *http.Response) error {
	switch res.StatusCode {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
		code := extractSafeCode(res.Body, res.StatusCode)
		msg := safeOwnerMessages[code]
		if msg == "" {
			msg = defaultMessageForStatus(res.StatusCode)
		}
		return &domain.OwnerError{Status: res.StatusCode, Code: code, Message: msg}
	default:
		return fmt.Errorf("%w: upstream status %d", domain.ErrUpstreamUnavailable, res.StatusCode)
	}
}

func extractSafeCode(body io.Reader, status int) string {
	var env errorEnvelope
	if err := json.NewDecoder(io.LimitReader(body, maxErrorBodyBytes)).Decode(&env); err == nil {
		code := strings.TrimSpace(env.Error.Code)
		if _, ok := safeOwnerMessages[code]; ok {
			return code
		}
	}
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	default:
		return "VALIDATION_FAILED"
	}
}

func defaultMessageForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Invalid request"
	case http.StatusNotFound:
		return "Resource not found"
	case http.StatusConflict:
		return "Operation conflict"
	default:
		return "Request rejected by owner service"
	}
}
