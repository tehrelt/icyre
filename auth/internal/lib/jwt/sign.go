package jwt

import (
	"time"

	"mzhn/auth/internal/domain/entity"

	"github.com/golang-jwt/jwt/v5"
)

func Sign(user *entity.UserClaims, ttl time.Duration, secret []byte) (string, error) {
	payload := claims{
		UserClaims: *user,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)

	signed, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return signed, nil
}
