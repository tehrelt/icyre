package handlers

import (
	"errors"
	"mzhn/auth/internal/domain"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/pkg/responses"

	"github.com/labstack/echo/v4"
)

func Register(as *authservice.AuthService) echo.HandlerFunc {
	type request struct {
		LastName   *string `json:"lastName"`
		FirstName  *string `json:"firstName"`
		MiddleName *string `json:"middleName"`
		Email      string  `json:"email"`
		Password   string  `json:"password"`
	}

	type response struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}

	return func(c echo.Context) error {
		var req request

		if err := c.Bind(&req); err != nil {
			return responses.Internal(c, err)
		}

		tokens, err := as.Register(c.Request().Context(), &dto.CreateUser{
			LastName:   req.LastName,
			FirstName:  req.FirstName,
			MiddleName: req.MiddleName,
			Email:      req.Email,
			Password:   req.Password,
		})
		if err != nil {
			if errors.Is(err, domain.ErrEmailTaken) {
				return responses.BadRequest(c, err)
			}
			return responses.Internal(c, err)
		}

		return c.JSON(200, &response{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		})
	}
}
