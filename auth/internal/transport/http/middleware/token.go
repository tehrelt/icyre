package middleware

import (
	"log/slog"
	"mzhn/auth/pkg/responses"
	"strings"

	"github.com/labstack/echo/v4"
)

func Token() func() echo.MiddlewareFunc {
	return func() echo.MiddlewareFunc {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {

				authHeader := c.Request().Header[echo.HeaderAuthorization]

				if len(authHeader) == 0 {
					return responses.Unauthorized(c)
				}

				bearer := authHeader[0]
				if bearer == "" {
					return responses.Unauthorized(c)
				}

				if !strings.HasPrefix(bearer, "Bearer ") {
					return responses.Unauthorized(c)
				}

				token := strings.Split(bearer, " ")[1]
				if token == "" {
					return responses.Unauthorized(c)
				}

				slog.Debug("get token", slog.String("token", token))
				c.Set(TOKEN, token)

				return next(c)
			}
		}
	}
}
