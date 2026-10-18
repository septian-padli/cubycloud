package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/auth"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*echo.Echo, string) {
	tempDir, err := os.MkdirTemp("", "cubycloud-auth-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	if err := os.MkdirAll(tempDir+"/db", 0755); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}

	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatalf("failed to connect db: %v", err)
	}

	jwtSecret := "test-secret-key-12345"
	jwtAuth := middleware.JWTAuth(jwtSecret)

	// Create test user
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 10)
	recoveryKey := "ABCDE-FGHIJ-KLMNP-QRSTU-VWXYZ-23456"
	recoveryHash := sha256.Sum256([]byte(strings.ReplaceAll(recoveryKey, "-", "")))
	recoveryHashStr := hex.EncodeToString(recoveryHash[:])
	user := &database.User{
		ID:              ulid.Make().String(),
		Username:        "admin",
		Email:           "admin@cubycloud.local",
		PasswordHash:    string(passwordHash),
		RecoveryKeyHash: &recoveryHashStr,
		Role:            "SUPER_ADMIN",
		QuotaBytes:      -1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	_, err = db.NewInsert().Model(user).Exec(context.Background())
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	e := echo.New()
	auth.RegisterRoutes(e, db, jwtSecret, jwtAuth)

	return e, tempDir
}

func TestLogin_Success(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	body, _ := json.Marshal(auth.LoginRequest{
		Identifier: "admin",
		Password:   "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res auth.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.User.Username != "admin" {
		t.Errorf("expected username 'admin', got '%s'", res.User.Username)
	}

	// Check cookie
	cookies := rec.Result().Cookies()
	var foundCookie bool
	for _, c := range cookies {
		if c.Name == "cuby_token" && c.Value != "" {
			foundCookie = true
			if !c.HttpOnly {
				t.Errorf("cookie should be HttpOnly")
			}
			break
		}
	}
	if !foundCookie {
		t.Errorf("expected cuby_token cookie to be set")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	body, _ := json.Marshal(auth.LoginRequest{
		Identifier: "admin",
		Password:   "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var foundCookie bool
	for _, c := range cookies {
		if c.Name == "cuby_token" && c.MaxAge < 0 {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Errorf("expected cuby_token cookie to be expired on logout")
	}
}
func TestResetPasswordRotatesAndConsumesRecoveryKey(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	requestReset := func(recoveryKey string) *httptest.ResponseRecorder {
		body, err := json.Marshal(auth.ResetPasswordRequest{
			Username:    "admin",
			RecoveryKey: recoveryKey,
			NewPassword: "newpassword123",
		})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	oldKey := "abcde-fghij-klmnp-qrstu-vwxyz-23456"
	first := requestReset(oldKey)
	if first.Code != http.StatusOK {
		t.Fatalf("first reset status = %d: %s", first.Code, first.Body.String())
	}
	var resetResponse auth.ResetPasswordResponse
	if err := json.Unmarshal(first.Body.Bytes(), &resetResponse); err != nil {
		t.Fatal(err)
	}
	if len(resetResponse.NewRecoveryKey) != 35 {
		t.Fatalf("new recovery key = %q, want six groups of five", resetResponse.NewRecoveryKey)
	}

	reused := requestReset(oldKey)
	if reused.Code != http.StatusUnauthorized {
		t.Fatalf("old key reuse status = %d, want 401", reused.Code)
	}

	// The returned grouped key is accepted without separators and case-insensitively.
	nextKey := strings.ToLower(strings.ReplaceAll(resetResponse.NewRecoveryKey, "-", ""))
	second := requestReset(nextKey)
	if second.Code != http.StatusOK {
		t.Fatalf("new ungrouped key reset status = %d: %s", second.Code, second.Body.String())
	}
}

func TestResetPasswordMigratesLegacyBcryptKey(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	legacyKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	legacyHash, err := bcrypt.GenerateFromPassword([]byte(legacyKey), 10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.NewUpdate().
		Model((*database.User)(nil)).
		Set("recovery_key_hash = ?", string(legacyHash)).
		Where("username = ?", "admin").
		Exec(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(auth.ResetPasswordRequest{
		Username: "admin", RecoveryKey: strings.ToUpper(legacyKey), NewPassword: "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy reset status = %d: %s", rec.Code, rec.Body.String())
	}
}
