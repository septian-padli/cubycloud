package search

import (
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

// RegisterRoutes registers global search endpoints on Echo instance.
func RegisterRoutes(e *echo.Echo, db *bun.DB, jwtAuth echo.MiddlewareFunc) *Handler {
	svc := NewService(db)
	h := NewHandler(svc)

	driveGroup := e.Group("/api/v1/drive", jwtAuth)
	driveGroup.GET("/search", h.Search)

	return h
}
