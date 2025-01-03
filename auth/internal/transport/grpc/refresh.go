package grpc

import (
	"context"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/pb/authpb"
)

func (s *Server) Refresh(ctx context.Context, in *authpb.RefreshRequest) (*authpb.RefreshResponse, error) {
	t, err := s.as.Refresh(ctx, &dto.Refresh{RefreshToken: in.RefreshToken})
	if err != nil {
		return nil, err
	}

	return &authpb.RefreshResponse{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
	}, nil
}
