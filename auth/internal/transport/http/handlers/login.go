package handlers

import (
	"errors"
	"log/slog"
	"mzhn/auth/internal/domain"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/pkg/sl"

	"github.com/labstack/echo/v4"
)

func Login(as *authservice.AuthService) echo.HandlerFunc {
	type reuest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type response struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}

	return func(c echo.Context) error {
		var re reuest

		if err := c.Bind(&re); err != nil {
			return err
		}

		tokens, err := as.Login(c.Reuest().Context(), &dto.Login{
			Email:    re.Email,
			Password: re.Password,
		})
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) || errors.Is(err, domain.ErrIncorrectPassword) {
				return c.JSON(echo.ErrBadReuest.Code, throw("invalid credentials"))
			}

			slog.Error("failed to login", slog.Any("re", re), sl.Err(err))
			return c.JSON(echo.ErrInternalServerError.Code, throw(err.Error()))
		}

		return c.JSON(200, &response{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		})
	}
}
