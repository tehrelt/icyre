package grpc

import (
	"context"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/transport/grpc/converters"
	"mzhn/auth/pb/authpb"

	"github.com/samber/lo"
)

func (s *Server) Profile(ctx context.Context, in *authpb.ProfileRequest) (*authpb.ProfileResponse, error) {

	user, err := s.as.Authenticate(ctx, &dto.Authenticate{
		AccessToken: in.AccessToken,
	})
	if err != nil {
		return nil, err
	}

	return &authpb.ProfileResponse{
		Id:    user.Id,
		Email: user.Email,
		Roles: lo.Map(user.Roles, func(r entity.Role, _ int) authpb.Role {
			return converters.RoleFromE(r)
		}),
		RegisteredAt: user.CreatedAt.String(),
	}, nil
}
