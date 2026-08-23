package admin

import (
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

// RegisterRoutes registers admin endpoints with JWT + Super Admin guard
func RegisterRoutes(e *echo.Echo, db *bun.DB, driver storage.StorageDriver, dataDirPath, dbPath string, jwtAuth, adminGuard echo.MiddlewareFunc) {
	repo := NewRepository(db)
	service := NewService(repo, driver, dataDirPath, dbPath)
	handler := NewHandler(service)

	// Admin routes - JWT auth + Super Admin guard required
	admin := e.Group("/api/v1/admin", jwtAuth, adminGuard)

	// System settings
	admin.GET("/settings", handler.GetSettings)
	admin.PUT("/settings", handler.UpdateSettings)

	// Storage nodes
	admin.GET("/storage-nodes", handler.GetNodes)
	admin.POST("/storage-nodes", handler.CreateLocalNode)
	admin.POST("/storage-nodes/cloud", handler.CreateCloudNode)
	admin.PUT("/storage-nodes/:id", handler.UpdateNode)
	admin.DELETE("/storage-nodes/:id", handler.DeleteNode)

	// Operations
	admin.POST("/storage-nodes/rescan", handler.RescanNodes)
	admin.POST("/storage-nodes/:id/disable", handler.DisableNode)
	admin.GET("/backup", handler.BackupDatabase)
}
