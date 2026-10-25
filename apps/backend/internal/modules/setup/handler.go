package setup

import (
	"net/http"
	"time"

	"github.com/cubycloud/backend/internal/httputil"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
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
	token, err := h.generateJWT(resp.UserID) // Use actual user ID for JWT subject
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
		Secure:   httputil.IsSecureCookie(c),
		MaxAge:   24 * 3600, // 24 hours
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": "SUPER_ADMIN",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(h.jwtSecret))
}
func RegisterRoutes(e *echo.Echo, db *bun.DB, storage storage.StorageDriver, setupGuard *middleware.SetupGuard, dataDirPath string, jwtSecret string) {
	h := NewHandler(db, storage, setupGuard, dataDirPath, jwtSecret)
	
	// Setup routes: status/scan open, complete protected by setup guard
	e.GET("/api/v1/setup/status", h.GetStatus)
	e.GET("/api/v1/setup/scan-nodes", h.ScanNodes)
	
	setupGroup := e.Group("/api/v1/setup", setupGuard.Middleware())
	setupGroup.POST("/complete", h.CompleteSetup)
}
