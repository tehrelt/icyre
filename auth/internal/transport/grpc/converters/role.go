package converters

import (
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/pb/authpb"
)

func RoleToE(role authpb.Role) entity.Role {
	switch role {
	case authpb.Role_ADMIN:
		return entity.RoleAdmin
	}

	return entity.RoleRegular
}

func RoleFromE(role entity.Role) authpb.Role {
	switch role {
	case entity.RoleAdmin:
		return authpb.Role_ADMIN
	}

	return authpb.Role_REGULAR
}
