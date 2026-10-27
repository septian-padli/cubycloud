package share

import (
	"time"

	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

// RegisterRoutes registers all share-related endpoints on the Echo instance.
func RegisterRoutes(e *echo.Echo, db *bun.DB, driver storage.StorageDriver, jwtAuth echo.MiddlewareFunc) (*Handler, *PublicHandler) {
	service := NewService(db, driver)
	h := NewHandler(service)
	ph := NewPublicHandler(service)

	// 1. User search for share autocomplete
	e.GET("/api/v1/share/users", h.SearchUsers, jwtAuth)

	// 2. Item shares management (JWT Auth, owner only)
	driveGroup := e.Group("/api/v1/drive", jwtAuth)
	driveGroup.GET("/items/:id/shares", h.GetItemShares)
	driveGroup.PUT("/items/:id/shares/users", h.PutUserGrants)
	driveGroup.DELETE("/items/:id/shares/users/:userId", h.DeleteUserGrant)
	driveGroup.POST("/items/:id/shares/links", h.CreateShareLink)
	driveGroup.DELETE("/items/:id/shares/links/:linkId", h.DeleteShareLink)

	// 3. Shared With Me endpoints (JWT Auth)
	sharedGroup := e.Group("/api/v1/drive/shared", jwtAuth)
	sharedGroup.GET("", h.ListSharedWithMe)
	sharedGroup.GET("/folders/:id", h.GetSharedFolder)
	sharedGroup.POST("/folders/:id/upload", h.UploadSharedFile)
	sharedGroup.POST("/folders/:id/folders", h.CreateSharedFolder)
	sharedGroup.PATCH("/items/:id", h.RenameSharedItem)
	sharedGroup.DELETE("/items/:id", h.DeleteSharedItem)
	sharedGroup.PATCH("/items/:id/move", h.MoveSharedItem)
	sharedGroup.GET("/items/:id/download", h.DownloadSharedItem)
	sharedGroup.GET("/folders/:id/download-zip", h.DownloadSharedFolderZip)

	// 4. Public share links (Public / optional JWT)
	publicGroup := e.Group("/api/v1/s")
	publicGroup.GET("/:token", ph.GetPublicShare)
	publicGroup.POST("/:token/unlock", ph.UnlockPublicShare, middleware.RateLimiterScoped("share_unlock", 10, time.Minute))
	publicGroup.GET("/:token/download", ph.DownloadPublicFile)
	publicGroup.GET("/:token/download-zip", ph.DownloadPublicZip)

	// Public share editor write endpoints (editor permission + JWT Auth required)
	publicGroup.POST("/:token/upload", ph.UploadPublicFile, jwtAuth)
	publicGroup.POST("/:token/folders", ph.CreatePublicFolder, jwtAuth)
	publicGroup.PATCH("/:token/items/:itemId", ph.RenamePublicItem, jwtAuth)
	publicGroup.DELETE("/:token/items/:itemId", ph.DeletePublicItem, jwtAuth)

	return h, ph
}
