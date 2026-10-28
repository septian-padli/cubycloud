package docs

import (
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers /api/docs and /api/docs/openapi.yaml endpoints,
// strictly guarded by JWT authentication and Super Admin authorization.
func RegisterRoutes(e *echo.Echo, jwtAuth, adminGuard echo.MiddlewareFunc) *Handler {
	h := NewHandler()
	g := e.Group("/api/docs", jwtAuth, adminGuard)
	g.GET("", h.GetDocsHTML)
	g.GET("/", h.GetDocsHTML)
	g.GET("/openapi.yaml", h.GetOpenAPISpec)
	g.GET("/assets/*", h.ServeAsset)
	return h
}
