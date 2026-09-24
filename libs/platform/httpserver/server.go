// Package httpserver provides the HTTP server, middleware and JSON helpers
// shared by ICYRE services.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Config configures the HTTP server.
type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	// RequestTimeout bounds the handler context (see Timeout middleware).
	RequestTimeout time.Duration
}

// withDefaults fills unset timeouts with safe values.
func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 15 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
	}
	return c
}

// Server wraps http.Server with a context-driven lifecycle.
type Server struct {
	srv *http.Server
	log *slog.Logger
}

// New creates a server for handler.
func New(cfg Config, handler http.Handler, log *slog.Logger) *Server {
	cfg = cfg.withDefaults()
	return &Server{
		log: log,
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
		},
	}
}

// Run listens until ctx is cancelled, then shuts down gracefully within
// shutdownTimeout. It returns nil on a clean shutdown.
func (s *Server) Run(ctx context.Context, shutdownTimeout time.Duration) error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.srv.Addr, err)
	}
	s.log.Info("http server listening", "addr", ln.Addr().String())

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	s.log.Info("http server shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := s.srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
