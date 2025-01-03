package grpc

import (
	"context"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/pb/authpb"
)

func (s *Server) Register(ctx context.Context, re *authpb.RegisterReuest) (*authpb.AuthResponse, error) {
	t, err := s.as.Register(ctx, &dto.CreateUser{
		Email:    re.Email,
		Password: re.Password,
	})
	if err != nil {
		return nil, err
	}

	return &authpb.AuthResponse{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
	}, nil
}
