package handlers

import (
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/services/authservice"
	mw "mzhn/auth/internal/transport/http/middleware"

	"github.com/labstack/echo/v4"
)

func Profile(as *authservice.AuthService) echo.HandlerFunc {

	type response struct {
		Id         string        `json:"id"`
		LastName   *string       `json:"lastName"`
		FirstName  *string       `json:"firstName"`
		MiddleName *string       `json:"middleName"`
		Email      string        `json:"email"`
		Roles      []entity.Role `json:"roles"`
	}

	return func(c echo.Context) error {
		user := c.Get(mw.USER).(*entity.User)

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
