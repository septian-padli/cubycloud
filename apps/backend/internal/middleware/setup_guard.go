package middleware

import (
	"net/http"
	"strings"
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
			path := c.Request().URL.Path

			// Always allow status check
			if path == "/api/v1/setup/status" {
				return next(c)
			}

			// If setup is NOT completed:
			if !g.IsCompleted() {
				// Allow setup endpoints
				if path == "/api/v1/setup/scan-nodes" || path == "/api/v1/setup/complete" || path == "/api/v1/setup/test-cloud" {
					return next(c)
				}

				// Any other route/API requires setup first -> 403 Forbidden
				if !isStaticAsset(path) && path != "/setup" && path != "/" {
					return echo.NewHTTPError(http.StatusForbidden, "setup required")
				}
			} else {
				// If setup IS completed:
				// Disallow /setup page and complete endpoint with 403 Forbidden
				if path == "/setup" || path == "/setup/" || path == "/api/v1/setup/complete" {
					return echo.NewHTTPError(http.StatusForbidden, "setup already completed")
				}
			}

			return next(c)
		}
	}
}

func isStaticAsset(path string) bool {
	return strings.HasPrefix(path, "/_app/") ||
		strings.HasPrefix(path, "/favicon") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".json")
}
