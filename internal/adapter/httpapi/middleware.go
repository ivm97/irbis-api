package httpapi

import (
	"github.com/labstack/echo/v4"
)

const (
	headerUser        = "X-API-User"
	headerPass        = "X-API-Password"
	headerWorkstation = "X-Irbis-Workstation"
)

func credentials(c echo.Context) (user, pass string) {
	return c.Request().Header.Get(headerUser), c.Request().Header.Get(headerPass)
}

func workstation(c echo.Context) string {
	return c.Request().Header.Get(headerWorkstation)
}
