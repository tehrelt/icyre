package authservice

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mzhn/auth/internal/domain"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/lib/jwt"
	"mzhn/auth/internal/storage"
	"mzhn/auth/pkg/sl"

	"github.com/samber/lo"
)

func (a *AuthService) parseAccessToken(token string) (*entity.UserClaims, error) {

	fn := "authservice.parseAccessToken"
	log := a.logger.With(sl.Method(fn))

	claims, err := jwt.Verify(token, a.cfg.Jwt.AccessSecret)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			log.Debug("verified token is expired")
			return nil, domain.ErrTokenExpired
		}

		log.Warn("invalid token", sl.Err(err))
		return nil, domain.ErrTokenInvalid
	}

	log.Debug("verified token", slog.Any("claims", claims))

	return claims, nil
}

func (a *AuthService) Authenticate(ctx context.Context, in *dto.Authenticate) (*entity.User, error) {

	fn := "authservice.Authenticate"
	log := a.logger.With(sl.Method(fn), slog.Any("in", in.Roles))

	claims, err := a.parseAccessToken(in.AccessToken)
	if err != nil {
		return nil, err
	}

	u, err := a.userProvider.Find(ctx, claims.Id)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Debug("user not found", slog.String("id", claims.Id), slog.String("email", claims.Email))
			return nil, fmt.Errorf("%s: %w", fn, domain.ErrUserNotFound)
		}

		log.Debug("cannot provide user", sl.Err(err))
		return nil, err
	}
	log.Debug("found user", slog.Any("user", u))

	if len(in.Roles) == 0 {
		log.Debug("auth request has no role guards")
		return u, nil
	}

	for _, ur := range u.Roles {
		if lo.Contains(in.Roles, ur) {
			log.Debug("user passed through role guard", slog.Any("role", ur))
			return u, nil
		}
	}

	return nil, fmt.Errorf("%s: %w", fn, domain.ErrInsufficientPermission)
}
