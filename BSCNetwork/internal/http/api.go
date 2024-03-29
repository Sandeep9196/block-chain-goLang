package http

import (
	"net/http"

	"bsc_network/internal/tools"
	"github.com/labstack/echo/v4"
)

func RegisterHandlers(g *echo.Group, version string) {
	g.GET("/healthcheck", healthcheck(version))
}

func healthcheck(version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return tools.JSON(c, http.StatusOK, tools.Success, "OK "+version)
	}
}
