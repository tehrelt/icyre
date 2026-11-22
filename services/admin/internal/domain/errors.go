// Package domain holds the Admin Service domain model, invariants, and errors.
package domain

import (
	"errors"
	"fmt"
)

// Domain sentinel errors.
var (
	ErrForbidden           = errors.New("admin: forbidden")
	ErrOperationNotFound   = errors.New("admin: operation not found")
	ErrIdempotencyConflict = errors.New("admin: idempotency key reused with different parameters")
	ErrCannotBlockSelf     = errors.New("admin: cannot moderate own account")
	ErrUpstreamUnavailable = errors.New("admin: upstream service unavailable")
)

// ValidationError reports field-level input validation failures.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

// OwnerError represents a deterministic 4xx rejection from an owner service
// (Catalog, Auth, or Media Ingest).
type OwnerError struct {
	Status  int
	Code    string
	Message string
}

func (e *OwnerError) Error() string {
	return fmt.Sprintf("owner error (%d %s): %s", e.Status, e.Code, e.Message)
}
