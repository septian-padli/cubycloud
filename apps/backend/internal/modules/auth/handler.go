package auth

import (
	"net/http"
	"time"

	"github.com/cubycloud/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

type Handler struct {
	service   *Service
	jwtSecret string
}

func NewHandler(service *Service, jwtSecret string) *Handler {
	return &Handler{
		service:   service,
		jwtSecret: jwtSecret,
	}
}

// Login handles POST /api/v1/auth/login
func (h *Handler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	user, err := h.service.Login(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}

	// Set HTTP-only cookie
	cookie := &http.Cookie{
		Name:     "cuby_token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   7 * 24 * 3600, // 7 days
	}
	c.SetCookie(cookie)
	usedBytes, _ := h.service.GetUserUsage(c.Request().Context(), user.ID)
	quotaBytes := user.QuotaBytes
	if user.Role == "SUPER_ADMIN" {
		quotaBytes = -1
	}
	return c.JSON(http.StatusOK, LoginResponse{
		User: UserResponse{
			ID:         user.ID,
			Username:   user.Username,
			Email:      user.Email,
			Role:       user.Role,
			QuotaBytes: quotaBytes,
			UsedBytes:  usedBytes,
		},
		Message: "login successful",
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *Handler) Logout(c echo.Context) error {
	cookie := &http.Cookie{
		Name:     "cuby_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request().TLS != nil,
		MaxAge:   -1,
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "logout successful",
	})
}

// Me handles GET /api/v1/auth/me
func (h *Handler) Me(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	user, err := h.service.GetUserByID(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user"})
	}
	if user == nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	usedBytes, _ := h.service.GetUserUsage(c.Request().Context(), user.ID)
	quotaBytes := user.QuotaBytes
	if user.Role == "SUPER_ADMIN" {
		quotaBytes = -1
	}
	return c.JSON(http.StatusOK, UserResponse{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		Role:       user.Role,
		QuotaBytes: quotaBytes,
		UsedBytes:  usedBytes,
	})
}

// ResetPassword handles POST /api/v1/auth/reset-password
func (h *Handler) ResetPassword(c echo.Context) error {
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Default username to admin if omitted
	if req.Username == "" {
		req.Username = "admin"
	}
	normalizedKey, validKey := normalizeRecoveryKey(req.RecoveryKey)
	if normalizedKey == "" || !validKey {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "recovery key must contain 30 alphanumeric characters in six groups of five"})
	}
	if len(req.NewPassword) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "new password must be at least 8 characters"})
	}

	res, err := h.service.ResetPassword(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, res)
}

// RegisterRoutes registers all auth routes
func RegisterRoutes(e *echo.Echo, db *bun.DB, jwtSecret string, jwtAuth echo.MiddlewareFunc) {
	service := NewService(db)
	handler := NewHandler(service, jwtSecret)

	authGroup := e.Group("/api/v1/auth")
	authGroup.POST("/login", handler.Login, middleware.RateLimiter(10, time.Minute))
	authGroup.POST("/logout", handler.Logout)
	authGroup.POST("/reset-password", handler.ResetPassword)
	authGroup.GET("/me", handler.Me, jwtAuth)
}
