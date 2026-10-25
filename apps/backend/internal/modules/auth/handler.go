package auth

import (
	"net/http"
	"time"

	"github.com/cubycloud/backend/internal/httputil"
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
		"sub":           user.ID,
		"role":          user.Role,
		"token_version": user.TokenVersion,
		"exp":           time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}

	usedBytes, err := h.service.GetUserUsage(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user usage"})
	}
	cookie := &http.Cookie{
		Name:     "cuby_token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   httputil.IsSecureCookie(c),
		MaxAge:   24 * 3600, // 24 hours
	}
	c.SetCookie(cookie)
	quotaBytes := user.QuotaBytes
	if user.Role == "SUPER_ADMIN" {
		quotaBytes = -1
	}
	return c.JSON(http.StatusOK, LoginResponse{
		User: UserResponse{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        user.Role,
			QuotaBytes:  quotaBytes,
			UsedBytes:   usedBytes,
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
		Secure:   httputil.IsSecureCookie(c),
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
	usedBytes, err := h.service.GetUserUsage(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user usage"})
	}
	quotaBytes := user.QuotaBytes
	if user.Role == "SUPER_ADMIN" {
		quotaBytes = -1
	}
	return c.JSON(http.StatusOK, UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		QuotaBytes:  quotaBytes,
		UsedBytes:   usedBytes,
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

// UpdateProfile handles PATCH /api/v1/auth/profile
func (h *Handler) UpdateProfile(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	user, err := h.service.UpdateProfile(c.Request().Context(), userID, &req)
	if err != nil {
		return err
	}

	usedBytes, err := h.service.GetUserUsage(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user usage"})
	}
	quotaBytes := user.QuotaBytes
	if user.Role == "SUPER_ADMIN" {
		quotaBytes = -1
	}

	return c.JSON(http.StatusOK, UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		QuotaBytes:  quotaBytes,
		UsedBytes:   usedBytes,
	})
}

// ChangePassword handles POST /api/v1/auth/change-password
func (h *Handler) ChangePassword(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := h.service.ChangePassword(c.Request().Context(), userID, &req); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "password changed successfully",
	})
}

// GetProfile handles GET /api/v1/auth/profile/:userId
func (h *Handler) GetProfile(c echo.Context) error {
	currentUserID, ok := c.Get("user_id").(string)
	if !ok || currentUserID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	targetUserID := c.Param("userId")
	if targetUserID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "userId is required"})
	}

	currentUserRole, _ := c.Get("user_role").(string)
	if currentUserRole == "" {
		currentUser, err := h.service.GetUserByID(c.Request().Context(), currentUserID)
		if err == nil && currentUser != nil {
			currentUserRole = currentUser.Role
		}
	}

	if currentUserRole != "SUPER_ADMIN" && targetUserID != currentUserID {
		return echo.NewHTTPError(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	targetUser, err := h.service.GetUserByID(c.Request().Context(), targetUserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user"})
	}
	if targetUser == nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "user not found"})
	}

	usedBytes, err := h.service.GetUserUsage(c.Request().Context(), targetUser.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user usage"})
	}
	quotaBytes := targetUser.QuotaBytes
	if targetUser.Role == "SUPER_ADMIN" {
		quotaBytes = -1
	}

	return c.JSON(http.StatusOK, UserResponse{
		ID:          targetUser.ID,
		Username:    targetUser.Username,
		Email:       targetUser.Email,
		DisplayName: targetUser.DisplayName,
		Role:        targetUser.Role,
		QuotaBytes:  quotaBytes,
		UsedBytes:   usedBytes,
	})
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
	authGroup.PATCH("/profile", handler.UpdateProfile, jwtAuth)
	authGroup.POST("/change-password", handler.ChangePassword, jwtAuth)
	authGroup.GET("/profile/:userId", handler.GetProfile, jwtAuth)
}
