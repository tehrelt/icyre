package jwt

import (
	"mzhn/auth/internal/domain/entity"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	entity.UserClaims
	jwt.RegisteredClaims
}
