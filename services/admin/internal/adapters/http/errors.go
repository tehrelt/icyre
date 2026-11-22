package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

const (
	codeOperationNotFound   = "OPERATION_NOT_FOUND"
	codeIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
	codeCannotBlockSelf     = "CANNOT_BLOCK_SELF"
	codeInvalidCursor       = "INVALID_CURSOR"
)

type requestError struct {
	status  int
	code    string
	message string
	details map[string]any
}

func (e *requestError) Error() string { return e.message }

func badRequest(msg string) error {
	return &requestError{status: http.StatusBadRequest, code: httpserver.CodeBadRequest, message: msg}
}

func invalidCursor() error {
	return &requestError{status: http.StatusBadRequest, code: codeInvalidCursor, message: "cursor is malformed"}
}

func invalidFields(fields map[string]string) error {
	details := make(map[string]any, len(fields))
	for k, v := range fields {
		details[k] = v
	}
	return &requestError{
		status:  http.StatusUnprocessableEntity,
		code:    httpserver.CodeValidation,
		message: "Request validation failed",
		details: map[string]any{"fields": details},
	}
}

func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var (
		reqErr   *requestError
		valErr   *domain.ValidationError
		ownerErr *domain.OwnerError
	)
	switch {
	case errors.As(err, &reqErr):
		httpserver.WriteError(w, r, reqErr.status, reqErr.code, reqErr.message, reqErr.details)
	case errors.As(err, &valErr):
		fields := make(map[string]any, len(valErr.Fields))
		for k, v := range valErr.Fields {
			fields[k] = v
		}
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
	case errors.As(err, &ownerErr):
		httpserver.WriteError(w, r, ownerErr.Status, ownerErr.Code, ownerErr.Message, nil)
	case errors.Is(err, domain.ErrForbidden):
		httpserver.WriteError(w, r, http.StatusForbidden, authn.CodeForbidden, "Insufficient permissions", nil)
	case errors.Is(err, domain.ErrOperationNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, codeOperationNotFound, "Moderation operation not found", nil)
	case errors.Is(err, domain.ErrIdempotencyConflict):
		httpserver.WriteError(w, r, http.StatusConflict, codeIdempotencyConflict, "Idempotency-Key already used with different parameters", nil)
	case errors.Is(err, domain.ErrCannotBlockSelf):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, codeCannotBlockSelf, "Cannot moderate your own account", nil)
	case errors.Is(err, domain.ErrUpstreamUnavailable):
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeUnavailable, "Upstream service is temporarily unavailable", nil)
	case errors.Is(err, context.DeadlineExceeded):
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeRequestTimeout, "Request timed out", nil)
	default:
		log.ErrorContext(r.Context(), "admin request failed", "error", err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, httpserver.CodeInternal, "Internal server error", nil)
	}
}
