package middleware

import (
	"net/http"
	"sync/atomic"

	"github.com/labstack/echo/v4"
)

type SetupGuard struct {
	completed atomic.Bool
}

func NewSetupGuard(isCompleted bool) *SetupGuard {
	g := &SetupGuard{}
	if isCompleted {
		g.completed.Store(true)
	}
	return g
}

func (g *SetupGuard) MarkCompleted() {
	g.completed.Store(true)
}

func (g *SetupGuard) IsCompleted() bool {
	return g.completed.Load()
}

func (g *SetupGuard) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/api/v1/setup/complete" {
				return next(c)
			}

			if !g.IsCompleted() && c.Path() != "/" && c.Path() != "/setup" {
				return c.Redirect(http.StatusTemporaryRedirect, "/setup")
			}

			return next(c)
		}
	}
}
