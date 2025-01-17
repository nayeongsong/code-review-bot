package web

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AppContext struct {
	echo.Context
	AppLogger *zap.Logger
}

func CreateAppContext(
	logger *zap.Logger,
) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &AppContext{c, logger}
			return next(cc)
		}
	}
}
