package responses

import "github.com/labstack/echo/v4"

type Payload map[string]interface{}

func Ok(c echo.Context, data Payload) error {
	return c.JSON(200, data)
}

func BadRequest(c echo.Context, err error) error {
	return c.JSON(400, Payload{"error": err.Error()})
}

func Unauthorized(c echo.Context) error {
	return c.JSON(401, Payload{"error": "Unauthorized"})
}

func Forbidden(c echo.Context) error {
	return c.JSON(403, Payload{"error": "Forbidden"})
}

func NotFound(c echo.Context) error {
	return c.JSON(404, Payload{"error": "Not Found"})
}

func Internal(c echo.Context, err error) error {
	return c.JSON(500, Payload{"error": err.Error()})
}
