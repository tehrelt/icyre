package handlers

import (
	"errors"
	"mzhn/auth/internal/domain"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/services/authservice"
	mw "mzhn/auth/internal/transport/http/middleware"

	"github.com/labstack/echo/v4"
)

func Refresh(as *authservice.AuthService) echo.HandlerFunc {

	type response struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}

	return func(c echo.Context) error {
		token := c.Get(mw.TOKEN)
		if token == nil {
			return c.JSON(echo.ErrBadRequest.Code, throw("token not found"))
		}

		tokens, err := as.Refresh(c.Request().Context(), &dto.Refresh{RefreshToken: token.(string)})
		if err != nil {
			if errors.Is(err, domain.ErrTokenExpired) || errors.Is(err, domain.ErrTokenInvalid) {
				return c.JSON(echo.ErrBadRequest.Code, throw("invalid token"))
			}

			return c.JSON(echo.ErrInternalServerError.Code, throw("internal server error"))
		}

		return c.JSON(200, &response{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		})
	}
}
