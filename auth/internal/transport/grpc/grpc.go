package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"mzhn/auth/internal/config"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/pb/authpb"
	"mzhn/auth/pkg/sl"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

var _ authpb.AuthServer = (*Server)(nil)

type Server struct {
	*authpb.UnimplementedAuthServer
	cfg *config.Config
	as  *authservice.AuthService
}

func New(cfg *config.Config, as *authservice.AuthService) *Server {
	return &Server{
		as:  as,
		cfg: cfg,
	}
}

func (s *Server) Run(ctx context.Context) error {

	log := slog.With(sl.Module("grpc"))

	server := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	host := s.cfg.Grpc.Host
	port := s.cfg.Grpc.Port
	addr := fmt.Sprintf("%s:%d", host, port)

	log.Info("starting grpc server", slog.String("addr", addr))

	if s.cfg.Grpc.UseReflection {
		log.Info("enabling reflection")
		reflection.Register(server)
	}

	authpb.RegisterAuthServer(server, s)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind port", slog.String("addr", addr), sl.Err(err))
		return err
	}

	go func() {
		if err := server.Serve(listener); err != nil {
			slog.Error("failed to serve", sl.Err(err))
			return
		}
	}()

	<-ctx.Done()
	log.Info("shutting down grpc server")
	server.GracefulStop()
	return nil
}
