package mystorage

import (
	"context"
	"net/http"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/labstack/echo/v4"
)

type QuickAccessResponseItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	SizeBytes  int64     `json:"size_bytes"`
	Source     string    `json:"source"`
	AccessedAt time.Time `json:"accessed_at"`
}

type ActivityResponseItem struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	ItemID    string    `json:"item_id"`
	FileName  string    `json:"file_name"`
	ItemKind  string    `json:"item_kind"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

type HomeResponse struct {
	QuickAccess []QuickAccessResponseItem `json:"quick_access"`
	Activities  []ActivityResponseItem    `json:"activities"`
}

// RecordAccess records or updates the last accessed time of an item for the user.
func (s *Service) RecordAccess(ctx context.Context, userID, itemID, source string) error {
	if source == "" {
		source = "my-storage"
	}
	return database.RecordRecentAccess(ctx, s.db.DB, userID, itemID, source)
}

// GetHome fetches up to 5 recently opened items and 5 latest upload/edit activities for the user.
func (s *Service) GetHome(ctx context.Context, userID string) (*HomeResponse, error) {
	resp := &HomeResponse{
		QuickAccess: make([]QuickAccessResponseItem, 0),
		Activities:  make([]ActivityResponseItem, 0),
	}

	// 1. Fetch up to 5 most recently accessed items
	accessRows, err := s.db.DB.QueryContext(ctx, `
		SELECT ra.item_id, ra.source, ra.accessed_at,
		       COALESCE(msi.name, oi.name) AS name,
		       COALESCE(msi.kind, oi.kind) AS kind,
		       COALESCE(msi.size_bytes, oi.size_bytes, 0) AS size_bytes
		FROM vfs_recent_access ra
		LEFT JOIN my_storage_items msi ON ra.source = 'my-storage' AND msi.id = ra.item_id AND msi.owner_id = ra.user_id AND msi.deleted_at IS NULL
		LEFT JOIN organization_items oi ON ra.source = 'organization' AND oi.id = ra.item_id AND oi.deleted_at IS NULL
		WHERE ra.user_id = ?
		  AND (msi.id IS NOT NULL OR oi.id IS NOT NULL)
		ORDER BY ra.accessed_at DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, err
	}
	defer accessRows.Close()

	for accessRows.Next() {
		var item QuickAccessResponseItem
		if err := accessRows.Scan(&item.ID, &item.Source, &item.AccessedAt, &item.Name, &item.Kind, &item.SizeBytes); err != nil {
			return nil, err
		}
		resp.QuickAccess = append(resp.QuickAccess, item)
	}
	if err := accessRows.Err(); err != nil {
		return nil, err
	}

	// 2. Fetch up to 5 latest distinct activities (one entry per item showing its most recent action)
	// For organization, only actions performed by the current user are included (al.user_id = ?).
	activityRows, err := s.db.DB.QueryContext(ctx, `
		WITH ranked_logs AS (
			SELECT al.id, al.action, al.item_id, al.item_name, al.item_kind, al.source, al.created_at,
			       ROW_NUMBER() OVER (PARTITION BY al.source, al.item_id ORDER BY al.created_at DESC) as rn
			FROM activity_logs al
			WHERE al.user_id = ?
			  AND al.action IN ('upload', 'edit')
		)
		SELECT rl.id, rl.action, rl.item_id,
		       COALESCE(msi.name, oi.name, rl.item_name) AS file_name,
		       COALESCE(msi.kind, oi.kind, rl.item_kind) AS item_kind,
		       rl.source, rl.created_at
		FROM ranked_logs rl
		LEFT JOIN my_storage_items msi ON rl.source = 'my-storage' AND msi.id = rl.item_id AND msi.owner_id = ? AND msi.deleted_at IS NULL
		LEFT JOIN organization_items oi ON rl.source = 'organization' AND oi.id = rl.item_id AND oi.deleted_at IS NULL
		WHERE rl.rn = 1
		  AND (
		      (rl.source = 'my-storage' AND msi.id IS NOT NULL)
		      OR
		      (rl.source = 'organization' AND oi.id IS NOT NULL)
		  )
		ORDER BY rl.created_at DESC
		LIMIT 5
	`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer activityRows.Close()

	for activityRows.Next() {
		var act ActivityResponseItem
		if err := activityRows.Scan(&act.ID, &act.Action, &act.ItemID, &act.FileName, &act.ItemKind, &act.Source, &act.CreatedAt); err != nil {
			return nil, err
		}
		resp.Activities = append(resp.Activities, act)
	}
	if err := activityRows.Err(); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetHome handles GET /api/v1/drive/home
func (h *Handler) GetHome(c echo.Context) error {
	userID := owner(c)
	resp, err := h.service.GetHome(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

type RecordAccessRequest struct {
	ItemID string `json:"item_id"`
	Source string `json:"source"`
}

// RecordAccess handles POST /api/v1/drive/access
func (h *Handler) RecordAccess(c echo.Context) error {
	userID := owner(c)
	var req RecordAccessRequest
	if err := c.Bind(&req); err != nil || req.ItemID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Source == "" {
		req.Source = "my-storage"
	}
	if req.Source != "my-storage" && req.Source != "organization" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid source")
	}

	if err := h.service.RecordAccess(c.Request().Context(), userID, req.ItemID, req.Source); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
