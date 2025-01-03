package authservice

import (
	"context"
	"fmt"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/lib/jwt"
	"mzhn/auth/pkg/sl"
)

func (a *AuthService) Refresh(ctx context.Context, req *dto.Refresh) (*dto.Tokens, error) {
	fn := "authservice.Refresh"
	log := a.logger.With(sl.Method(fn))

	log.Debug("refreshing user's session")

	claims, err := jwt.Verify(req.RefreshToken, a.cfg.Jwt.RefreshSecret)
	if err != nil {
		log.Error("refresh token invalid", sl.Err(err))
		if err := a.sessions.Delete(ctx, req.RefreshToken); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	if err := a.sessions.Check(ctx, claims.Id, req.RefreshToken); err != nil {
		log.Error("session not found", sl.Err(err))
		return nil, err
	}

	tokens, err := a.generateJwtPair(claims)
	if err != nil {
		log.Error("cannot generate jwt", sl.Err(err))
		return nil, err
	}

	if err := a.sessions.Delete(ctx, claims.Id); err != nil {
		return nil, err
	}

	if err := a.sessions.Save(ctx, claims.Id, tokens.RefreshToken); err != nil {
		log.Error("save session error", sl.Err(err))
		return nil, err
	}

	return tokens, nil
}
