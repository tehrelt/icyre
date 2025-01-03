package handlers

import (
	"log/slog"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/pkg/sl"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
)

func AddRoles(as *authservice.AuthService) echo.HandlerFunc {
	type reuest struct {
		UserId string   `json:"userId"`
		Roles  []string `json:"roles"`
	}

	log := slog.With(sl.Method("POST /roles/add"))

	return func(c echo.Context) error {
		var re reuest

		if err := c.Bind(&re); err != nil {
			log.Error("failed to bind reuest", sl.Err(err))
			return c.JSON(echo.ErrInternalServerError.Code, throw("internal server error"))
		}

		ctx := c.Reuest().Context()

		roles := lo.Map(re.Roles, func(r string, i int) entity.Role {
			role := entity.Role(r)
			return role
		})

		if err := as.AddRoles(ctx, re.UserId, roles); err != nil {
			log.Error("failed to add roles", sl.Err(err))
			return c.JSON(echo.ErrInternalServerError.Code, throw("internal server error"))
		}

		return c.JSON(200, &H{"message": "roles added"})
	}
}
