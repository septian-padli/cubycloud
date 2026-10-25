package docs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/docs"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupTestDB(t *testing.T) (*bun.DB, string) {
	t.Helper()
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))

	db, err := database.Connect(tempDir)
	require.NoError(t, err)

	return db, tempDir
}

func makeToken(t *testing.T, secret, userID, role string, tokenVersion int) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":           userID,
		"role":          role,
		"token_version": tokenVersion,
		"exp":           time.Now().Add(time.Hour).Unix(),
	})
	str, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return str
}

func TestDocsAccessControl(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	jwtSecret := "test-secret-at-least-32-bytes-super-secret"

	// Create a regular user and a super admin user in DB
	regUser := database.User{
		ID:           "user-regular-1",
		Username:     "reguser",
		Email:        "user@example.com",
		PasswordHash: "dummyhash",
		Role:         "REGULAR",
		TokenVersion: 1,
	}
	_, err := db.NewInsert().Model(&regUser).Exec(ctx)
	require.NoError(t, err)

	adminUser := database.User{
		ID:           "user-admin-1",
		Username:     "adminuser",
		Email:        "admin@example.com",
		PasswordHash: "dummyhash",
		Role:         "SUPER_ADMIN",
		TokenVersion: 1,
	}
	_, err = db.NewInsert().Model(&adminUser).Exec(ctx)
	require.NoError(t, err)

	jwtAuth := middleware.JWTAuth(jwtSecret, db)
	superAdminGuard := middleware.SuperAdminGuard(db)

	e := echo.New()
	docs.RegisterRoutes(e, jwtAuth, superAdminGuard)

	regToken := makeToken(t, jwtSecret, regUser.ID, regUser.Role, regUser.TokenVersion)
	adminToken := makeToken(t, jwtSecret, adminUser.ID, adminUser.Role, adminUser.TokenVersion)

	tests := []struct {
		name       string
		path       string
		cookieVal  string
		authHeader string
		wantCode   int
		wantBody   string
	}{
		// 1. Unauthenticated -> 401
		{
			name:     "Unauthenticated /api/docs",
			path:     "/api/docs",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "Unauthenticated /api/docs/openapi.yaml",
			path:     "/api/docs/openapi.yaml",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "Unauthenticated /api/docs/assets/swagger-ui.css",
			path:     "/api/docs/assets/swagger-ui.css",
			wantCode: http.StatusUnauthorized,
		},

		// 2. Regular user -> 403
		{
			name:      "Regular user via cookie /api/docs",
			path:      "/api/docs",
			cookieVal: regToken,
			wantCode:  http.StatusForbidden,
		},
		{
			name:       "Regular user via bearer /api/docs",
			path:       "/api/docs",
			authHeader: "Bearer " + regToken,
			wantCode:   http.StatusForbidden,
		},
		{
			name:      "Regular user via cookie /api/docs/openapi.yaml",
			path:      "/api/docs/openapi.yaml",
			cookieVal: regToken,
			wantCode:  http.StatusForbidden,
		},

		// 3. Super Admin -> 200
		{
			name:      "Super Admin via cookie /api/docs",
			path:      "/api/docs",
			cookieVal: adminToken,
			wantCode:  http.StatusOK,
			wantBody:  "CubyCloud API Documentation",
		},
		{
			name:       "Super Admin via bearer /api/docs",
			path:       "/api/docs",
			authHeader: "Bearer " + adminToken,
			wantCode:   http.StatusOK,
			wantBody:   "CubyCloud API Documentation",
		},
		{
			name:      "Super Admin /api/docs/openapi.yaml",
			path:      "/api/docs/openapi.yaml",
			cookieVal: adminToken,
			wantCode:  http.StatusOK,
			wantBody:  "openapi:",
		},
		{
			name:      "Super Admin /api/docs/assets/swagger-ui.css",
			path:      "/api/docs/assets/swagger-ui.css",
			cookieVal: adminToken,
			wantCode:  http.StatusOK,
			wantBody:  "swagger-ui",
		},
		{
			name:      "Super Admin /api/docs/assets/swagger-ui-bundle.js",
			path:      "/api/docs/assets/swagger-ui-bundle.js",
			cookieVal: adminToken,
			wantCode:  http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.cookieVal != "" {
				req.AddCookie(&http.Cookie{
					Name:  "cuby_token",
					Value: tc.cookieVal,
				})
			}
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantCode, rec.Code)
			if tc.wantBody != "" {
				assert.Contains(t, rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestSwaggerUITryItOutCookieConfiguration(t *testing.T) {
	h := docs.NewHandler()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.GetDocsHTML(c)
	require.NoError(t, err)

	body := rec.Body.String()
	// Must contain same-origin credentials for Try it out
	assert.True(t, strings.Contains(body, "req.credentials = 'same-origin'"), "Swagger UI must configure credentials: same-origin for Try it out cookie forwarding")
	assert.True(t, strings.Contains(body, "/api/docs/openapi.yaml"), "Swagger UI must point to /api/docs/openapi.yaml")
	assert.True(t, strings.Contains(body, "swagger-ui-bundle.js"), "Swagger UI must load local bundle script")
	assert.True(t, strings.Contains(body, "swagger-ui.css"), "Swagger UI must load local stylesheet")
}
