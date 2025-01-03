package authservice

import (
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/lib/jwt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (a *AuthService) hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		a.cfg.Bcrypt.Cost,
	)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func (a *AuthService) comparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (a *AuthService) generateJwtPair(claims *entity.UserClaims) (*dto.Tokens, error) {
	accessToken, err := jwt.Sign(claims, time.Duration(a.cfg.Jwt.AccessTTL)*time.Minute, []byte(a.cfg.Jwt.AccessSecret))
	if err != nil {
		return nil, err
	}

	refreshToken, err := jwt.Sign(claims, time.Duration(a.cfg.Jwt.RefreshTTL)*time.Minute, []byte(a.cfg.Jwt.RefreshSecret))
	if err != nil {
		return nil, err
	}

	return &dto.Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
