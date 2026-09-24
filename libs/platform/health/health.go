// Package health serves liveness and readiness endpoints.
//
//	GET /health/live  — the process is up (never touches dependencies).
//	GET /health/ready — every registered dependency check passes.
package health

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
)

// CheckFunc reports whether a dependency is usable.
type CheckFunc func(ctx context.Context) error

// Checker aggregates readiness checks.
type Checker struct {
	timeout  time.Duration
	mu       sync.RWMutex
	checks   map[string]CheckFunc
	draining atomic.Bool
}

// New returns a Checker whose checks run with the given per-request timeout.
func New(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Checker{timeout: timeout, checks: map[string]CheckFunc{}}
}

// Add registers a named readiness check.
func (c *Checker) Add(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = fn
}

// Drain makes readiness fail so load balancers stop routing new traffic
// while the service shuts down.
func (c *Checker) Drain() { c.draining.Store(true) }

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Register mounts the endpoints on mux.
func (c *Checker) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", c.live)
	mux.HandleFunc("GET /health/ready", c.ready)
}

func (c *Checker) live(w http.ResponseWriter, _ *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, response{Status: "ok"})
}

func (c *Checker) ready(w http.ResponseWriter, r *http.Request) {
	if c.draining.Load() {
		httpserver.WriteJSON(w, http.StatusServiceUnavailable, response{Status: "draining"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), c.timeout)
	defer cancel()

	c.mu.RLock()
	checks := make(map[string]CheckFunc, len(c.checks))
	for k, v := range c.checks {
		checks[k] = v
	}
	c.mu.RUnlock()

	results := make(map[string]string, len(checks))
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		failed bool
	)
	for name, fn := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := "ok"
			if err := fn(ctx); err != nil {
				// Only a generic status is exposed; details go to logs/metrics.
				status = "unavailable"
				mu.Lock()
				failed = true
				mu.Unlock()
			}
			mu.Lock()
			results[name] = status
			mu.Unlock()
		}()
	}
	wg.Wait()

	if failed {
		httpserver.WriteJSON(w, http.StatusServiceUnavailable, response{Status: "unavailable", Checks: results})
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, response{Status: "ok", Checks: results})
}
