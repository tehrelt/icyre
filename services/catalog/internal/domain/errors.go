// Package domain holds the Catalog model: artists, albums, tracks, genres
// and their invariants. It has no knowledge of HTTP, PostgreSQL or Kafka.
package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Not-found errors returned by repositories and use cases.
var (
	ErrArtistNotFound = errors.New("artist not found")
	ErrAlbumNotFound  = errors.New("album not found")
	ErrTrackNotFound  = errors.New("track not found")
	ErrGenreNotFound  = errors.New("genre not found")
)

// Business rule violations.
var (
	ErrInvalidStatusTransition = errors.New("invalid track status transition")
	ErrTrackPositionTaken      = errors.New("track position is already taken on this album")
	ErrTrackDeleted            = errors.New("deleted track cannot be modified")
)

// ValidationError lists invalid fields with human-readable reasons.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e.Fields[k])
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// validator accumulates field problems.
type validator map[string]string

func (v validator) check(ok bool, field, msg string) {
	if !ok {
		if _, exists := v[field]; !exists {
			v[field] = msg
		}
	}
}

func (v validator) err() error {
	if len(v) == 0 {
		return nil
	}
	return &ValidationError{Fields: v}
}

// ReferenceError reports that an entity referenced by Field does not exist
// (e.g. creating a track for a missing album). It unwraps to the not-found
// error of the referenced entity.
type ReferenceError struct {
	Field string
	Err   error
}

func (e *ReferenceError) Error() string { return fmt.Sprintf("%s: %v", e.Field, e.Err) }
func (e *ReferenceError) Unwrap() error { return e.Err }
