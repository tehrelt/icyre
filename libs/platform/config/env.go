// Package config reads service configuration from environment variables.
//
// Every accessor records a problem instead of failing immediately, so a
// service reports all misconfigured variables at once via Err.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env reads typed values from environment variables.
type Env struct {
	lookup func(string) (string, bool)
	errs   []error
}

// New returns an Env backed by the process environment.
func New() *Env {
	return &Env{lookup: os.LookupEnv}
}

// NewFromMap returns an Env backed by a map. Useful in tests.
func NewFromMap(m map[string]string) *Env {
	return &Env{lookup: func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}}
}

func (e *Env) get(key string) (string, bool) {
	v, ok := e.lookup(key)
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	return v, v != ""
}

func (e *Env) fail(key, format string, args ...any) {
	e.errs = append(e.errs, fmt.Errorf("%s: %s", key, fmt.Sprintf(format, args...)))
}

// String returns the value of key or def when it is unset or empty.
func (e *Env) String(key, def string) string {
	if v, ok := e.get(key); ok {
		return v
	}
	return def
}

// RequiredString returns the value of key and records an error when it is missing.
func (e *Env) RequiredString(key string) string {
	v, ok := e.get(key)
	if !ok {
		e.fail(key, "required variable is not set")
	}
	return v
}

// Int returns key parsed as an integer or def.
func (e *Env) Int(key string, def int) int {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		e.fail(key, "expected integer, got %q", v)
		return def
	}
	return n
}

// Bool returns key parsed as a boolean or def.
func (e *Env) Bool(key string, def bool) bool {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		e.fail(key, "expected boolean, got %q", v)
		return def
	}
	return b
}

// Duration returns key parsed with time.ParseDuration or def.
func (e *Env) Duration(key string, def time.Duration) time.Duration {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		e.fail(key, "expected duration (e.g. 5s), got %q", v)
		return def
	}
	return d
}

// Strings returns key split by commas or def.
func (e *Env) Strings(key string, def []string) []string {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Err returns all recorded problems joined, or nil.
func (e *Env) Err() error {
	return errors.Join(e.errs...)
}
