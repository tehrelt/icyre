package handlers

import (
	"errors"
	"log/slog"
	"mzhn/auth/internal/domain"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/services/authservice"
	mw "mzhn/auth/internal/transport/http/middleware"
	"mzhn/auth/pkg/responses"
	"mzhn/auth/pkg/sl"

	"github.com/labstack/echo/v4"
)

func Authenticate(as *authservice.AuthService) echo.HandlerFunc {

	type request struct {
		Roles []entity.Role `json:"roles"`
	}

	type response struct {
		Id         string        `json:"id"`
		LastName   *string       `json:"lastName"`
		FirstName  *string       `json:"firstName"`
		MiddleName *string       `json:"middleName"`
		Email      string        `json:"email"`
		Roles      []entity.Role `json:"roles"`
	}

	return func(c echo.Context) error {
		token := c.Get(mw.TOKEN)
		if token == nil {
			slog.Error("token not found")
			return responses.BadRequest(c, errors.New("token not found"))
		}

		var req request

		if err := c.Bind(&req); err != nil {
			slog.Error("failed to bind request", sl.Err(err))
			return responses.BadRequest(c, err)
		}

		ctx := c.Request().Context()
		user, err := as.Authenticate(ctx, &dto.Authenticate{
			AccessToken: token.(string),
			Roles:       req.Roles,
		})
		if err != nil {
			slog.Error("failed to authenticate token", sl.Err(err))

			if errors.Is(err, domain.ErrTokenInvalid) {
				return responses.Unauthorized(c)
			} else if errors.Is(err, domain.ErrTokenExpired) {
				return responses.Unauthorized(c)
			} else if errors.Is(err, domain.ErrUserNotFound) {
				return responses.Unauthorized(c)
			} else if errors.Is(err, domain.ErrInsufficientPermission) {
				return responses.Forbidden(c)
			}

			return responses.Internal(c, err)
		}

		slog.Debug("user authenticated", slog.Any("user", user))

		return c.JSON(200, &response{
			Id:         user.Id,
			LastName:   user.LastName,
			FirstName:  user.FirstName,
			MiddleName: user.MiddleName,
			Email:      user.Email,
			Roles:      user.Roles,
		})
	}
}
