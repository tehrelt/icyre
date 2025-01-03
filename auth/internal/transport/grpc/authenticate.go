package grpc

import (
	"context"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/transport/grpc/converters"
	"mzhn/auth/pb/authpb"
)

func (s *Server) Authenticate(ctx context.Context, in *authpb.AuthenticateRequest) (*authpb.AuthenticateResponse, error) {

	roles := make([]entity.Role, 0, len(in.Roles))

	for _, role := range in.Roles {
		roles = append(roles, converters.RoleToE(role))
	}

	u, err := s.as.Authenticate(ctx, &dto.Authenticate{
		AccessToken: in.AccessToken,
		Roles:       roles,
	})
	if err != nil {
		return nil, err
	}

	return &authpb.AuthenticateResponse{Approved: u != nil}, nil
}
