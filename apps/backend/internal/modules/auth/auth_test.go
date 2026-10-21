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
	jwtAuth := middleware.JWTAuth(jwtSecret, db)

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
		QuotaBytes:      1, // Simulate stale stored quota; role must keep API quota unlimited.
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

	if res.User.QuotaBytes != -1 {
		t.Errorf("Super Admin quota_bytes = %d, want -1", res.User.QuotaBytes)
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

func TestMe_Success(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	// 1. Login to obtain auth cookie and user info
	loginBody, _ := json.Marshal(auth.LoginRequest{
		Identifier: "admin",
		Password:   "password123",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginRes auth.LoginResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginRes); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}

	userID := loginRes.User.ID
	if userID == "" {
		t.Fatalf("expected non-empty user ID")
	}

	// 2. Insert items into my_storage_items to verify deduplicated usage calculation
	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, role) VALUES ('other-user', 'other', 'other@example.com', 'hash', 'REGULAR')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, modified_at)
		VALUES
			('item-1', ?, 'file1.bin', 'file', 1000, 'blob-a', CURRENT_TIMESTAMP),
			('item-2', ?, 'file1_copy.bin', 'file', 1000, 'blob-a', CURRENT_TIMESTAMP),
			('item-3', ?, 'file2.bin', 'file', 2500, 'blob-b', CURRENT_TIMESTAMP),
			('item-4', ?, 'file3.bin', 'file', 500, NULL, CURRENT_TIMESTAMP),
			('item-5', ?, 'folder1', 'folder', 9999, NULL, CURRENT_TIMESTAMP),
			('item-6', 'other-user', 'other.bin', 'file', 10000, 'blob-other', CURRENT_TIMESTAMP)
	`, userID, userID, userID, userID, userID)
	if err != nil {
		t.Fatalf("failed to insert test storage items: %v", err)
	}

	// 3. Request GET /api/v1/auth/me with auth cookies
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	for _, c := range loginRec.Result().Cookies() {
		meReq.AddCookie(c)
	}
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", meRec.Code, meRec.Body.String())
	}

	var userRes auth.UserResponse
	if err := json.Unmarshal(meRec.Body.Bytes(), &userRes); err != nil {
		t.Fatalf("failed to parse me response: %v", err)
	}

	if userRes.ID != userID {
		t.Errorf("expected user ID '%s', got '%s'", userID, userRes.ID)
	}

	if userRes.QuotaBytes != -1 {
		t.Errorf("Super Admin quota_bytes = %d, want -1", userRes.QuotaBytes)
	}

	// Deduplicated usage: blob-a (1000) + blob-b (2500) + item-4 (500) = 4000
	if userRes.UsedBytes != 4000 {
		t.Errorf("expected used_bytes 4000, got %d", userRes.UsedBytes)
	}

	// Verify raw JSON response contains used_bytes key
	var raw map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse raw json: %v", err)
	}
	if _, ok := raw["used_bytes"]; !ok {
		t.Errorf("expected response to contain 'used_bytes' field")
	}
}

func TestLogin_RateLimiter(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	loginBody, _ := json.Marshal(auth.LoginRequest{
		Identifier: "nonexistent",
		Password:   "wrongpassword",
	})

	// The rate limiter is set to 10 requests per minute
	for range 10 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", "99.88.77.66")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		// Should be 401 (invalid credentials), not 429 yet
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for request within limit, got %d", rec.Code)
		}
	}

	// 11th request from same IP must be rejected with 429 Too Many Requests
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "99.88.77.66")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests for 11th request, got %d: %s", rec.Code, rec.Body.String())
	}
}

func loginHelper(t *testing.T, e *echo.Echo, identifier, password string) (*http.Cookie, auth.UserResponse) {
	t.Helper()
	body, _ := json.Marshal(auth.LoginRequest{
		Identifier: identifier,
		Password:   password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed (%s): %d %s", identifier, rec.Code, rec.Body.String())
	}
	var loginRes auth.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &loginRes); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	var authCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "cuby_token" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatalf("cuby_token cookie not found in login response")
	}
	return authCookie, loginRes.User
}

func TestUpdateProfile_DisplayNameOnly(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	cookie, _ := loginHelper(t, e, "admin", "password123")

	displayName := "Administrator Boss"
	patchBody, _ := json.Marshal(auth.UpdateProfileRequest{
		DisplayName: &displayName,
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(patchBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res auth.UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.DisplayName != "Administrator Boss" {
		t.Errorf("expected display_name 'Administrator Boss', got '%s'", res.DisplayName)
	}

	// Verify GET /api/v1/auth/me returns updated display_name
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(cookie)
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me request failed: %d", meRec.Code)
	}
	var meRes auth.UserResponse
	_ = json.Unmarshal(meRec.Body.Bytes(), &meRes)
	if meRes.DisplayName != "Administrator Boss" {
		t.Errorf("expected me display_name 'Administrator Boss', got '%s'", meRes.DisplayName)
	}
}

func TestUpdateProfile_RejectsInvalidDisplayName(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	cookie, _ := loginHelper(t, e, "admin", "password123")

	for _, displayName := range []string{"  ", strings.Repeat("界", 101)} {
		body, _ := json.Marshal(auth.UpdateProfileRequest{DisplayName: &displayName})
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid display name, got %d: %s", rec.Code, rec.Body.String())
		}
	}
}

func TestUpdateProfile_EmailChangeRequiresPassword(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	cookie, _ := loginHelper(t, e, "admin", "password123")

	newEmail := "new-admin@cubycloud.local"

	// 1. Missing current_password -> 400 Bad Request
	bodyNoPass, _ := json.Marshal(auth.UpdateProfileRequest{
		Email: &newEmail,
	})
	req1 := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(bodyNoPass))
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req1.AddCookie(cookie)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without current password, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 2. Wrong current_password -> 400 Bad Request
	bodyWrongPass, _ := json.Marshal(auth.UpdateProfileRequest{
		Email:           &newEmail,
		CurrentPassword: "wrong-password",
	})
	req2 := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(bodyWrongPass))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req2.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with wrong current password, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 3. Invalid email format -> 400 Bad Request
	invalidEmail := "not-an-email"
	bodyInvalidEmail, _ := json.Marshal(auth.UpdateProfileRequest{
		Email:           &invalidEmail,
		CurrentPassword: "password123",
	})
	req3 := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(bodyInvalidEmail))
	req3.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req3.AddCookie(cookie)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with invalid email, got %d: %s", rec3.Code, rec3.Body.String())
	}

	// 4. Correct password and valid email -> 200 OK
	bodyCorrect, _ := json.Marshal(auth.UpdateProfileRequest{
		Email:           &newEmail,
		CurrentPassword: "password123",
	})
	req4 := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(bodyCorrect))
	req4.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req4.AddCookie(cookie)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct password, got %d: %s", rec4.Code, rec4.Body.String())
	}
	var res auth.UserResponse
	_ = json.Unmarshal(rec4.Body.Bytes(), &res)
	if res.Email != newEmail {
		t.Errorf("expected email '%s', got '%s'", newEmail, res.Email)
	}
}

func TestUpdateProfile_EmailDuplicateCaseInsensitive(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	db, _ := database.Connect(tempDir)
	defer db.Close()

	// Insert second user
	passHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 10)
	user2 := &database.User{
		ID:           ulid.Make().String(),
		Username:     "user2",
		Email:        "user2@cubycloud.local",
		PasswordHash: string(passHash),
		Role:         "REGULAR",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err := db.NewInsert().Model(user2).Exec(context.Background())
	if err != nil {
		t.Fatalf("failed to insert second user: %v", err)
	}
	var defaultDisplayName string
	if err := db.QueryRowContext(context.Background(), "SELECT display_name FROM users WHERE id = ?", user2.ID).Scan(&defaultDisplayName); err != nil {
		t.Fatal(err)
	}
	if defaultDisplayName != user2.Username {
		t.Fatalf("insert default display_name = %q, want username %q", defaultDisplayName, user2.Username)
	}
	_, err = db.ExecContext(context.Background(), `
INSERT INTO users(id, username, email, password_hash, role, quota_bytes)
VALUES('duplicate-email-user', 'duplicate', 'USER2@CUBYCLOUD.LOCAL', 'hash', 'REGULAR', -1)`)
	if err == nil {
		t.Fatal("expected database to reject a case-insensitive duplicate email")
	}
	cookie, _ := loginHelper(t, e, "admin", "password123")

	// Attempt to set email to existing email in different case
	dupEmail := "USER2@CUBYCLOUD.LOCAL"
	body, _ := json.Marshal(auth.UpdateProfileRequest{
		Email:           &dupEmail,
		CurrentPassword: "password123",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate case-insensitive email, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateProfile_ConcurrentEmailUpdatesRemainUnique(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)
	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	passHash, err := bcrypt.GenerateFromPassword([]byte("password123"), 10)
	if err != nil {
		t.Fatal(err)
	}
	user := &database.User{
		ID:           ulid.Make().String(),
		Username:     "user2",
		Email:        "user2@cubycloud.local",
		PasswordHash: string(passHash),
		Role:         "REGULAR",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := db.NewInsert().Model(user).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	adminCookie, _ := loginHelper(t, e, "admin", "password123")
	userCookie, _ := loginHelper(t, e, "user2", "password123")
	email := "shared-update@cubycloud.local"
	start := make(chan struct{})
	results := make(chan int, 2)

	for _, cookie := range []*http.Cookie{adminCookie, userCookie} {
		go func(cookie *http.Cookie) {
			<-start
			body, _ := json.Marshal(auth.UpdateProfileRequest{
				Email:           &email,
				CurrentPassword: "password123",
			})
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/profile", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			results <- rec.Code
		}(cookie)
	}
	close(start)

	successes, conflicts := 0, 0
	for range 2 {
		switch status := <-results; status {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent profile update status %d", status)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one successful update and one conflict; got %d successes, %d conflicts", successes, conflicts)
	}
	var matchingEmails int
	if err := db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER(?)", email,
	).Scan(&matchingEmails); err != nil {
		t.Fatal(err)
	}
	if matchingEmails != 1 {
		t.Fatalf("case-insensitive email count = %d, want 1", matchingEmails)
	}
}

func TestChangePassword_ValidationAndSuccess(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	cookie, _ := loginHelper(t, e, "admin", "password123")

	// 1. Password mismatch -> 400
	bodyMismatch, _ := json.Marshal(auth.ChangePasswordRequest{
		CurrentPassword: "password123",
		NewPassword:     "newpassword123",
		ConfirmPassword: "differentpassword",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(bodyMismatch))
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req1.AddCookie(cookie)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mismatch, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 2. Too short (<8 chars) -> 400
	bodyShort, _ := json.Marshal(auth.ChangePasswordRequest{
		CurrentPassword: "password123",
		NewPassword:     "short",
		ConfirmPassword: "short",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(bodyShort))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req2.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 3. Passwords exceeding bcrypt's 72-byte limit -> 400
	longPassword := strings.Repeat("a", 73)
	bodyLong, _ := json.Marshal(auth.ChangePasswordRequest{
		CurrentPassword: "password123",
		NewPassword:     longPassword,
		ConfirmPassword: longPassword,
	})
	reqLong := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(bodyLong))
	reqLong.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	reqLong.AddCookie(cookie)
	recLong := httptest.NewRecorder()
	e.ServeHTTP(recLong, reqLong)
	if recLong.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for password longer than 72 bytes, got %d: %s", recLong.Code, recLong.Body.String())
	}

	// 3. Wrong current password -> 400
	bodyWrong, _ := json.Marshal(auth.ChangePasswordRequest{
		CurrentPassword: "wrongpassword",
		NewPassword:     "newpassword123",
		ConfirmPassword: "newpassword123",
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(bodyWrong))
	req3.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req3.AddCookie(cookie)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong current password, got %d: %s", rec3.Code, rec3.Body.String())
	}

	// 4. Valid password change -> 200
	bodyValid, _ := json.Marshal(auth.ChangePasswordRequest{
		CurrentPassword: "password123",
		NewPassword:     "newpassword123",
		ConfirmPassword: "newpassword123",
	})
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(bodyValid))
	req4.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req4.AddCookie(cookie)
	rec4 := httptest.NewRecorder()
	e.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid change, got %d: %s", rec4.Code, rec4.Body.String())
	}

	// Old password cannot log in
	loginOldBody, _ := json.Marshal(auth.LoginRequest{
		Identifier: "admin",
		Password:   "password123",
	})
	reqOld := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginOldBody))
	reqOld.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recOld := httptest.NewRecorder()
	e.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with old password, got %d", recOld.Code)
	}

	// New password can log in
	loginNewBody, _ := json.Marshal(auth.LoginRequest{
		Identifier: "admin",
		Password:   "newpassword123",
	})
	reqNew := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginNewBody))
	reqNew.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recNew := httptest.NewRecorder()
	e.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("expected 200 with new password, got %d: %s", recNew.Code, recNew.Body.String())
	}
}

func TestSessionInvalidation_AfterPasswordChange(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	// 1. Initial login
	cookie, _ := loginHelper(t, e, "admin", "password123")

	// 2. Active session works
	meReq1 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq1.AddCookie(cookie)
	meRec1 := httptest.NewRecorder()
	e.ServeHTTP(meRec1, meReq1)
	if meRec1.Code != http.StatusOK {
		t.Fatalf("expected 200 before password change, got %d", meRec1.Code)
	}

	// 3. Change password
	bodyChange, _ := json.Marshal(auth.ChangePasswordRequest{
		CurrentPassword: "password123",
		NewPassword:     "brandnewpass999",
		ConfirmPassword: "brandnewpass999",
	})
	changeReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(bodyChange))
	changeReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	changeReq.AddCookie(cookie)
	changeRec := httptest.NewRecorder()
	e.ServeHTTP(changeRec, changeReq)
	if changeRec.Code != http.StatusOK {
		t.Fatalf("password change failed: %d: %s", changeRec.Code, changeRec.Body.String())
	}

	// 4. OLD token must now return 401 Unauthorized ("invalid or expired session")
	meReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq2.AddCookie(cookie)
	meRec2 := httptest.NewRecorder()
	e.ServeHTTP(meRec2, meReq2)
	if meRec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for invalidated session, got %d: %s", meRec2.Code, meRec2.Body.String())
	}
}

func TestGetProfile_RBAC(t *testing.T) {
	e, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	db, _ := database.Connect(tempDir)
	defer db.Close()

	// Insert regular user
	passHash, _ := bcrypt.GenerateFromPassword([]byte("regular123"), 10)
	regularUser := &database.User{
		ID:           ulid.Make().String(),
		Username:     "regularuser",
		Email:        "regular@cubycloud.local",
		DisplayName:  "Regular User",
		PasswordHash: string(passHash),
		Role:         "REGULAR",
		QuotaBytes:   1073741824, // 1 GB
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err := db.NewInsert().Model(regularUser).Exec(context.Background())
	if err != nil {
		t.Fatalf("failed to insert regular user: %v", err)
	}

	adminCookie, adminUser := loginHelper(t, e, "admin", "password123")
	regularCookie, regUserRes := loginHelper(t, e, "regularuser", "regular123")

	// 1. Regular user accessing their own profile -> 200 OK
	regOwnReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile/"+regUserRes.ID, nil)
	regOwnReq.AddCookie(regularCookie)
	regOwnRec := httptest.NewRecorder()
	e.ServeHTTP(regOwnRec, regOwnReq)
	if regOwnRec.Code != http.StatusOK {
		t.Fatalf("expected regular user to access own profile (200), got %d: %s", regOwnRec.Code, regOwnRec.Body.String())
	}

	// 2. Regular user accessing admin's profile -> 403 Forbidden
	regAdminReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile/"+adminUser.ID, nil)
	regAdminReq.AddCookie(regularCookie)
	regAdminRec := httptest.NewRecorder()
	e.ServeHTTP(regAdminRec, regAdminReq)
	if regAdminRec.Code != http.StatusForbidden {
		t.Fatalf("expected regular user accessing other user to be 403, got %d: %s", regAdminRec.Code, regAdminRec.Body.String())
	}

	// 3. Super Admin accessing regular user's profile -> 200 OK
	adminViewRegReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile/"+regUserRes.ID, nil)
	adminViewRegReq.AddCookie(adminCookie)
	adminViewRegRec := httptest.NewRecorder()
	e.ServeHTTP(adminViewRegRec, adminViewRegReq)
	if adminViewRegRec.Code != http.StatusOK {
		t.Fatalf("expected super admin to view other profile (200), got %d: %s", adminViewRegRec.Code, adminViewRegRec.Body.String())
	}
	var regProfile auth.UserResponse
	_ = json.Unmarshal(adminViewRegRec.Body.Bytes(), &regProfile)
	if regProfile.DisplayName != "Regular User" {
		t.Errorf("expected display_name 'Regular User', got '%s'", regProfile.DisplayName)
	}
	if regProfile.QuotaBytes != 1073741824 {
		t.Errorf("expected quota_bytes 1073741824, got %d", regProfile.QuotaBytes)
	}

	// 4. Super Admin accessing own profile -> 200 OK
	adminOwnReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile/"+adminUser.ID, nil)
	adminOwnReq.AddCookie(adminCookie)
	adminOwnRec := httptest.NewRecorder()
	e.ServeHTTP(adminOwnRec, adminOwnReq)
	if adminOwnRec.Code != http.StatusOK {
		t.Fatalf("expected super admin to view own profile (200), got %d: %s", adminOwnRec.Code, adminOwnRec.Body.String())
	}

	// 5. Accessing non-existent user profile -> 404 Not Found
	nonExistentReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile/non-existent-id", nil)
	nonExistentReq.AddCookie(adminCookie)
	nonExistentRec := httptest.NewRecorder()
	e.ServeHTTP(nonExistentRec, nonExistentReq)
	if nonExistentRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent profile, got %d: %s", nonExistentRec.Code, nonExistentRec.Body.String())
	}
}
