package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

func setupTestDB(t *testing.T) *bun.DB {
	t.Helper()
	tempDir := t.TempDir()
	if err := os.MkdirAll(tempDir+"/db", 0755); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatalf("database.Connect failed: %v", err)
	}
	return db
}

func TestSuperAdminGuard(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	adminUser := &database.User{
		ID:           "admin-user-id",
		Username:     "superadmin",
		Email:        "admin@example.com",
		PasswordHash: "dummy-hash",
		Role:         "SUPER_ADMIN",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := db.NewInsert().Model(adminUser).Exec(ctx); err != nil {
		t.Fatalf("failed to insert admin user: %v", err)
	}

	regularUser := &database.User{
		ID:           "regular-user-id",
		Username:     "regularuser",
		Email:        "regular@example.com",
		PasswordHash: "dummy-hash",
		Role:         "REGULAR",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := db.NewInsert().Model(regularUser).Exec(ctx); err != nil {
		t.Fatalf("failed to insert regular user: %v", err)
	}

	tests := []struct {
		name         string
		userID       interface{}
		userRole     interface{}
		wantStatus   int
		wantNextCall bool
	}{
		{
			name:         "Case 1: unauthenticated (user_id nil) -> 401",
			userID:       nil,
			userRole:     nil,
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
		{
			name:         "Case 2: user_role = REGULAR di context -> 403",
			userID:       "some-user-id",
			userRole:     "REGULAR",
			wantStatus:   http.StatusForbidden,
			wantNextCall: false,
		},
		{
			name:         "Case 3: user_role = SUPER_ADMIN di context -> 200 (next dipanggil)",
			userID:       "some-user-id",
			userRole:     "SUPER_ADMIN",
			wantStatus:   http.StatusOK,
			wantNextCall: true,
		},
		{
			name:         "Case 4: user_role tidak ada di context, query DB menemukan role SUPER_ADMIN -> 200",
			userID:       "admin-user-id",
			userRole:     nil,
			wantStatus:   http.StatusOK,
			wantNextCall: true,
		},
		{
			name:         "Case 5: user_role tidak ada di context, query DB menemukan role REGULAR -> 403",
			userID:       "regular-user-id",
			userRole:     nil,
			wantStatus:   http.StatusForbidden,
			wantNextCall: false,
		},
		{
			name:         "Case 6: user_role tidak ada di context, query DB no rows -> 401",
			userID:       "non-existent-user-id",
			userRole:     nil,
			wantStatus:   http.StatusUnauthorized,
			wantNextCall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if tc.userID != nil {
				c.Set("user_id", tc.userID)
			}
			if tc.userRole != nil {
				c.Set("user_role", tc.userRole)
			}

			nextCalled := false
			next := func(c echo.Context) error {
				nextCalled = true
				return c.NoContent(http.StatusOK)
			}

			guard := middleware.SuperAdminGuard(db)
			err := guard(next)(c)

			if tc.wantNextCall {
				if err != nil {
					t.Fatalf("expected next to succeed, got error: %v", err)
				}
				if !nextCalled {
					t.Fatal("expected next handler to be called, but it was not")
				}
			} else {
				if nextCalled {
					t.Fatal("expected next handler NOT to be called")
				}
				if err == nil {
					t.Fatalf("expected error with status %d, got nil", tc.wantStatus)
				}
				he, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("expected *echo.HTTPError, got %T: %v", err, err)
				}
				if he.Code != tc.wantStatus {
					t.Errorf("status code = %d, want %d", he.Code, tc.wantStatus)
				}
			}
		})
	}
}

func TestJWTAuth_ExtractUserRole(t *testing.T) {
	jwtSecret := "test-secret-key"
	authMiddleware := middleware.JWTAuth(jwtSecret)

	t.Run("token with role claim sets user_id and user_role", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":  "user-123",
			"role": "SUPER_ADMIN",
			"exp":  time.Now().Add(time.Hour).Unix(),
		})
		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		var capturedUserID, capturedUserRole interface{}
		next := func(c echo.Context) error {
			capturedUserID = c.Get("user_id")
			capturedUserRole = c.Get("user_role")
			return c.NoContent(http.StatusOK)
		}

		err = authMiddleware(next)(c)
		if err != nil {
			t.Fatalf("expected JWTAuth to succeed, got: %v", err)
		}

		if capturedUserID != "user-123" {
			t.Errorf("captured user_id = %v, want 'user-123'", capturedUserID)
		}
		if capturedUserRole != "SUPER_ADMIN" {
			t.Errorf("captured user_role = %v, want 'SUPER_ADMIN'", capturedUserRole)
		}
	})

	t.Run("legacy token without role claim sets user_id only", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user-456",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		var capturedUserID, capturedUserRole interface{}
		next := func(c echo.Context) error {
			capturedUserID = c.Get("user_id")
			capturedUserRole = c.Get("user_role")
			return c.NoContent(http.StatusOK)
		}

		err = authMiddleware(next)(c)
		if err != nil {
			t.Fatalf("expected JWTAuth to succeed, got: %v", err)
		}

		if capturedUserID != "user-456" {
			t.Errorf("captured user_id = %v, want 'user-456'", capturedUserID)
		}
		if capturedUserRole != nil {
			t.Errorf("captured user_role = %v, want nil", capturedUserRole)
		}
	})
}
