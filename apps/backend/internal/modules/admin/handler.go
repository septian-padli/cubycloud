package admin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

type Handler struct {
	service     *Service
	dbPath      string
	dataDirPath string
}

func NewHandler(db *bun.DB, storage storage.StorageDriver, dataDirPath, dbPath string) *Handler {
	return &Handler{
		service:     NewService(db, storage, dataDirPath),
		dbPath:      dbPath,
		dataDirPath: dataDirPath,
	}
}

// GetSettings returns current system settings
func (h *Handler) GetSettings(c echo.Context) error {
	resp, err := h.service.GetSystemSettings(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get settings: %v", err),
		})
	}
	return c.JSON(http.StatusOK, resp)
}

// UpdateSettings updates system settings
func (h *Handler) UpdateSettings(c echo.Context) error {
	var req UpdateSettingsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if err := h.service.UpdateSystemSettings(c.Request().Context(), &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Return updated settings
	resp, err := h.service.GetSystemSettings(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": "settings updated but failed to retrieve",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// GetStorageNodes returns all storage nodes
func (h *Handler) GetStorageNodes(c echo.Context) error {
	nodes, err := h.service.GetStorageNodes(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to get storage nodes: %v", err),
		})
	}

	return c.JSON(http.StatusOK, StorageNodesListResponse{
		Nodes: nodes,
	})
}

// DisableStorageNode disables a storage node
func (h *Handler) DisableStorageNode(c echo.Context) error {
	nodeID := c.Param("id")
	if nodeID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
			"error": "node id required",
		})
	}

	if err := h.service.DisableStorageNode(c.Request().Context(), nodeID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "node disabled successfully",
	})
}

// RescanStorageNodes rescans local storage
func (h *Handler) RescanStorageNodes(c echo.Context) error {
	nodes, err := h.service.RescanStorageNodes(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to rescan nodes: %v", err),
		})
	}

	return c.JSON(http.StatusOK, StorageNodesListResponse{
		Nodes: nodes,
	})
}

// DownloadBackup streams database backup
func (h *Handler) DownloadBackup(c echo.Context) error {
	dbFile, err := os.Open(h.dbPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": "failed to open database file",
		})
	}
	defer dbFile.Close()

	stat, err := dbFile.Stat()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": "failed to stat database file",
		})
	}

	c.Response().Header().Set("Content-Disposition", "attachment; filename=cubycloud.db")
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	_, err = io.Copy(c.Response().Writer, dbFile)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"error": "failed to stream database",
		})
	}

	return nil
}

// CreateLocalNode adds a new local disk storage node
func (h *Handler) CreateLocalNode(c echo.Context) error {
	var req CreateLocalNodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	node, err := h.service.CreateLocalNode(c.Request().Context(), &req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		if strings.Contains(err.Error(), "does not exist") {
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to create node"})
	}

	return c.JSON(http.StatusCreated, node)
}

// CreateCloudNode adds a new S3-compatible cloud storage node
func (h *Handler) CreateCloudNode(c echo.Context) error {
	var req CreateCloudNodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	node, err := h.service.CreateCloudNode(c.Request().Context(), &req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to create node"})
	}

	return c.JSON(http.StatusCreated, node)
}

// UpdateNode updates storage node capacity and/or enabled status
func (h *Handler) UpdateNode(c echo.Context) error {
	nodeID := c.Param("id")
	if nodeID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "node ID required"})
	}

	var req UpdateNodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.service.UpdateNode(c.Request().Context(), nodeID, &req); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "node not found"})
		}
		if strings.Contains(err.Error(), "capacity") {
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to update node"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "node updated successfully"})
}

// DeleteNode soft-deletes a storage node
func (h *Handler) DeleteNode(c echo.Context) error {
	nodeID := c.Param("id")
	if nodeID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "node ID required"})
	}

	if err := h.service.DeleteNode(c.Request().Context(), nodeID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "node not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to delete node"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "node deleted successfully"})
}

// RegisterRoutes registers all admin endpoints with JWT auth + Super Admin guard
func RegisterRoutes(e *echo.Echo, db *bun.DB, storage storage.StorageDriver, dataDirPath, dbPath string, jwtAuth, superAdminGuard echo.MiddlewareFunc) {
	handler := NewHandler(db, storage, dataDirPath, dbPath)

	// Admin group with JWT auth + Super Admin guard
	admin := e.Group("/api/v1/admin", jwtAuth, superAdminGuard)
	
	// Settings
	admin.GET("/settings", handler.GetSettings)
	admin.PUT("/settings", handler.UpdateSettings)
	
	// Storage nodes
	admin.GET("/storage-nodes", handler.GetStorageNodes)
	admin.POST("/storage-nodes", handler.CreateLocalNode)
	admin.POST("/storage-nodes/cloud", handler.CreateCloudNode)
	admin.PUT("/storage-nodes/:id", handler.UpdateNode)
	admin.PUT("/storage-nodes/:id/disable", handler.DisableStorageNode)
	admin.DELETE("/storage-nodes/:id", handler.DeleteNode)
	admin.POST("/storage-nodes/rescan", handler.RescanStorageNodes)
	
	// Backup
	admin.GET("/backup", handler.DownloadBackup)
}
