package setup

import (
	"net/http"

	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

type Handler struct {
	service   *Service
	jwtSecret string
}

func NewHandler(db *bun.DB, storage storage.StorageDriver, setupGuard *middleware.SetupGuard, dataDirPath string, jwtSecret string) *Handler {
	return &Handler{
		service:   NewService(db, storage, setupGuard, dataDirPath),
		jwtSecret: jwtSecret,
	}
}

func (h *Handler) CompleteSetup(c echo.Context) error {
	var req CompleteSetupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	resp, err := h.service.CompleteSetup(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	// Generate JWT
	token, err := h.generateJWT(resp.RecoveryKey) // ponytail: use user ID after auth module lands
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "generate token failed")
	}

	// Set HTTP-only cookie
	cookie := &http.Cookie{
		Name:     "cuby_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   7 * 24 * 3600, // 7 days
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetStatus(c echo.Context) error {
	resp, err := h.service.GetSetupStatus(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get setup status")
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) ScanNodes(c echo.Context) error {
	resp, err := h.service.ScanStorageNodes(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "scan failed")
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) generateJWT(userID string) (string, error) {
	// ponytail: stub until auth module with proper JWT signing lands; use jwt_secret_version from settings
	return "stub_jwt_token_" + userID, nil
}

func RegisterRoutes(e *echo.Echo, db *bun.DB, storage storage.StorageDriver, setupGuard *middleware.SetupGuard, dataDirPath string, jwtSecret string) {
	h := NewHandler(db, storage, setupGuard, dataDirPath, jwtSecret)
	e.GET("/api/v1/setup/status", h.GetStatus)
	e.GET("/api/v1/setup/scan-nodes", h.ScanNodes)
	e.POST("/api/v1/setup/complete", h.CompleteSetup)
}
