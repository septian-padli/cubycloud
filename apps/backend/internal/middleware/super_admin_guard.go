package middleware

import (
	"net/http"

	"github.com/cubycloud/backend/internal/database"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

// SuperAdminGuard middleware ensures the user has super_admin role
func SuperAdminGuard(db *bun.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user ID from JWT context (set by auth middleware)
			userID := c.Get("user_id")
			if userID == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
					"error": "authentication required",
				})
			}

			// Query user role from database
			var user database.User
			err := db.NewSelect().
				Model(&user).
				Where("id = ?", userID).
				Scan(c.Request().Context())
			
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
					"error": "failed to verify user role",
				})
			}

			if user.Role != "SUPER_ADMIN" {
				return echo.NewHTTPError(http.StatusForbidden, map[string]string{
					"error": "super admin access required",
				})
			}

			return next(c)
		}
	}
}
