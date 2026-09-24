package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
)

// Stable, machine-readable error codes of the Catalog API.
const (
	codeArtistNotFound          = "ARTIST_NOT_FOUND"
	codeAlbumNotFound           = "ALBUM_NOT_FOUND"
	codeTrackNotFound           = "TRACK_NOT_FOUND"
	codeGenreNotFound           = "GENRE_NOT_FOUND"
	codeInvalidStatusTransition = "INVALID_STATUS_TRANSITION"
	codeTrackPositionTaken      = "TRACK_POSITION_TAKEN"
	codeTrackDeleted            = "TRACK_DELETED"
	codeInvalidCursor           = "INVALID_CURSOR"
)

var notFound = []struct {
	err  error
	code string
	msg  string
}{
	{domain.ErrArtistNotFound, codeArtistNotFound, "Artist not found"},
	{domain.ErrAlbumNotFound, codeAlbumNotFound, "Album not found"},
	{domain.ErrTrackNotFound, codeTrackNotFound, "Track not found"},
	{domain.ErrGenreNotFound, codeGenreNotFound, "Genre not found"},
}

// requestError is a transport-level problem found before the use case runs.
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

func invalidFields(fields map[string]string) error {
	details := make(map[string]any, len(fields))
	for k, v := range fields {
		details[k] = v
	}
	return &requestError{status: http.StatusUnprocessableEntity, code: httpserver.CodeValidation, message: "Request validation failed", details: map[string]any{"fields": details}}
}

// writeError maps application and domain errors to HTTP (error-model.md).
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var (
		reqErr *requestError
		valErr *domain.ValidationError
		refErr *domain.ReferenceError
	)
	switch {
	case errors.As(err, &reqErr):
		httpserver.WriteError(w, r, reqErr.status, reqErr.code, reqErr.message, reqErr.details)
		return
	case errors.As(err, &valErr):
		fields := make(map[string]any, len(valErr.Fields))
		for k, v := range valErr.Fields {
			fields[k] = v
		}
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, httpserver.CodeValidation, "Request validation failed", map[string]any{"fields": fields})
		return
	case errors.As(err, &refErr):
		// A referenced entity in the body is missing: the request is
		// well-formed but cannot be processed, hence 422 rather than 404.
		for _, nf := range notFound {
			if errors.Is(refErr.Err, nf.err) {
				httpserver.WriteError(w, r, http.StatusUnprocessableEntity, nf.code, nf.msg, map[string]any{"field": refErr.Field})
				return
			}
		}
	case errors.Is(err, domain.ErrInvalidStatusTransition):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, codeInvalidStatusTransition, err.Error(), nil)
		return
	case errors.Is(err, domain.ErrTrackDeleted):
		httpserver.WriteError(w, r, http.StatusUnprocessableEntity, codeTrackDeleted, "Deleted track cannot be modified", nil)
		return
	case errors.Is(err, domain.ErrTrackPositionTaken):
		httpserver.WriteError(w, r, http.StatusConflict, codeTrackPositionTaken, "Track number is already used on this disc", nil)
		return
	case errors.Is(err, context.DeadlineExceeded):
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, httpserver.CodeRequestTimeout, "Request timed out", nil)
		return
	}

	for _, nf := range notFound {
		if errors.Is(err, nf.err) {
			httpserver.WriteError(w, r, http.StatusNotFound, nf.code, nf.msg, nil)
			return
		}
	}

	log.ErrorContext(r.Context(), "catalog request failed", "error", err)
	httpserver.WriteError(w, r, http.StatusInternalServerError, httpserver.CodeInternal, "Internal server error", nil)
}
