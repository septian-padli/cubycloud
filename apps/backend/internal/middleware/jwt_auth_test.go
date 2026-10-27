package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func TestJWTAuthUsesCurrentDatabaseRole(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	const userID = "role-change-user"
	user := &database.User{
		ID:           userID,
		Username:     "role-change-user",
		Email:        "role-change@example.com",
		PasswordHash: "dummy-hash",
		Role:         "SUPER_ADMIN",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := db.NewInsert().Model(user).Exec(ctx); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	const jwtSecret = "test-secret-key"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":            userID,
		"role":           "SUPER_ADMIN",
		"token_version":  0,
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, "REGULAR", userID); err != nil {
		t.Fatalf("change user role: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	c := e.NewContext(req, httptest.NewRecorder())
	guardCalled := false
	handler := middleware.JWTAuth(jwtSecret, db)(middleware.SuperAdminGuard(db)(func(c echo.Context) error {
		guardCalled = true
		return c.NoContent(http.StatusOK)
	}))

	err = handler(c)
	if guardCalled {
		t.Fatal("demoted user reached the super admin handler")
	}
	if role := c.Get("user_role"); role != "REGULAR" {
		t.Fatalf("context user_role = %v, want REGULAR", role)
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("middleware error = %v, want 403", err)
	}
}
