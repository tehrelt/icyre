package grpc

import (
	"context"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/pb/authpb"
)

func (s *Server) Login(ctx context.Context, in *authpb.LoginRequest) (*authpb.AuthResponse, error) {
	tokens, err := s.as.Login(ctx, &dto.Login{
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		return nil, err
	}

	return &authpb.AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}
