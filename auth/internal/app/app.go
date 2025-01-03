package app

import (
	"context"
	"log/slog"
	"mzhn/auth/internal/config"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Server interface {
	Run(ctx context.Context) error
}

type App struct {
	servers []Server
	cfg     *config.Config
}

func newApp(cfg *config.Config, servers []Server) *App {
	return &App{
		servers: servers,
		cfg:     cfg,
	}
}

func (a *App) Run(ctx context.Context) {

	if len(a.servers) == 0 {
		slog.Error("no server to run")
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg := &sync.WaitGroup{}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	for _, server := range a.servers {
		wg.Add(1)
		go func(s Server) {
			defer wg.Done()
			if err := s.Run(ctx); err != nil {
				return
			}
		}(server)
	}

	s := <-sig
	slog.Info("execution stopped by signal", slog.String("signal", s.String()))

	cancel()
	wg.Wait()
}
