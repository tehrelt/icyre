package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

// Stable, service-independent error codes (specs/api/error-model.md).
// Services add domain-specific codes such as TRACK_NOT_FOUND.
const (
	CodeBadRequest      = "BAD_REQUEST"
	CodeValidation      = "VALIDATION_FAILED"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeInternal        = "INTERNAL"
	CodeUnavailable     = "SERVICE_UNAVAILABLE"
	CodeRequestTimeout  = "REQUEST_TIMEOUT"
	CodeUnsupportedType = "UNSUPPORTED_MEDIA_TYPE"
)

// ErrorBody is the public error envelope.
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

// ErrorPayload describes one API error.
type ErrorPayload struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// WriteJSON writes v with status as application/json.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the standard error envelope.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	WriteJSON(w, status, ErrorBody{Error: ErrorPayload{
		Code:      code,
		Message:   message,
		RequestID: requestid.From(r.Context()),
		Details:   details,
	}})
}

// MaxBodyBytes limits JSON request bodies.
const MaxBodyBytes = 1 << 20

// DecodeError is returned by DecodeJSON for malformed input.
type DecodeError struct{ msg string }

func (e *DecodeError) Error() string { return e.msg }

// DecodeJSON strictly decodes a single JSON object into dst.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return &DecodeError{"request body is empty"}
		case errors.As(err, &syntaxErr):
			return &DecodeError{fmt.Sprintf("malformed JSON at offset %d", syntaxErr.Offset)}
		case errors.As(err, &typeErr):
			return &DecodeError{fmt.Sprintf("field %q has the wrong type", typeErr.Field)}
		case errors.As(err, &maxErr):
			return &DecodeError{"request body is too large"}
		default:
			return &DecodeError{err.Error()}
		}
	}
	if dec.More() {
		return &DecodeError{"request body must contain a single JSON object"}
	}
	return nil
}
