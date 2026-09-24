// Package shutdown handles OS signals and ordered resource cleanup.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// NotifyContext returns a context cancelled on SIGINT or SIGTERM.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

// Stack closes registered resources in reverse order of registration
// (LIFO), so dependencies opened first are closed last.
type Stack struct {
	log *slog.Logger
	mu  sync.Mutex
	fns []named
}

type named struct {
	name string
	fn   func(context.Context) error
}

// NewStack returns an empty Stack.
func NewStack(log *slog.Logger) *Stack { return &Stack{log: log} }

// Add registers a cleanup function.
func (s *Stack) Add(name string, fn func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fns = append(s.fns, named{name, fn})
}

// AddFunc registers a cleanup function that cannot fail.
func (s *Stack) AddFunc(name string, fn func()) {
	s.Add(name, func(context.Context) error { fn(); return nil })
}

// Close runs every cleanup function in LIFO order, even if some fail.
func (s *Stack) Close(ctx context.Context) error {
	s.mu.Lock()
	fns := s.fns
	s.fns = nil
	s.mu.Unlock()

	var errs []error
	for i := len(fns) - 1; i >= 0; i-- {
		f := fns[i]
		if err := f.fn(ctx); err != nil {
			s.log.ErrorContext(ctx, "shutdown step failed", "step", f.name, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", f.name, err))
			continue
		}
		s.log.InfoContext(ctx, "shutdown step done", "step", f.name)
	}
	return errors.Join(errs...)
}
