package admin

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

type auditLogEntry struct {
	bun.BaseModel `bun:"table:audit_logs,alias:al"`

	ID           string  `bun:"id" json:"id"`
	UserID       *string `bun:"user_id" json:"user_id"`
	IPAddress    *string `bun:"ip_address" json:"ip_address"`
	UserAgent    *string `bun:"user_agent" json:"user_agent"`
	Method       string  `bun:"method" json:"method"`
	Path         string  `bun:"path" json:"path"`
	StatusCode   int     `bun:"status_code" json:"status_code"`
	Action       string  `bun:"action" json:"action"`
	Details      *string `bun:"details" json:"details"`
	ErrorMessage *string `bun:"error_message" json:"error_message"`
	CreatedAt    string  `bun:"created_at" json:"created_at"`
}

func (h *Handler) GetAuditLogs(c echo.Context) error {
	limit := 50
	if raw := c.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 1 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	offset := 0
	if raw := c.QueryParam("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	logs := make([]auditLogEntry, 0)
	query := h.service.db.NewSelect().Model(&logs).Order("al.created_at DESC").Limit(limit).Offset(offset)
	if action := c.QueryParam("action"); action != "" {
		query = query.Where("al.action = ?", action)
	}
	if userID := c.QueryParam("user_id"); userID != "" {
		query = query.Where("al.user_id = ?", userID)
	}
	if err := query.Scan(c.Request().Context()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to retrieve audit logs"})
	}
	return c.JSON(http.StatusOK, logs)
}
