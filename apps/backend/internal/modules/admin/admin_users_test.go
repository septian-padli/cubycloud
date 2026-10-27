package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestAdminUsers_CreateAndUpdateQuota(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Set system default quota to 50 GB
	fifty := 50
	err := handler.service.UpdateSystemSettings(ctx, &UpdateSettingsRequest{
		DefaultQuota: &QuotaPolicy{Mode: "limited", GB: fifty},
	})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}

	// 2. Create user with mode DEFAULT -> should snapshot 50 GB
	userDefault, err := handler.service.CreateUser(ctx, &CreateUserRequest{
		Username:  "siti",
		Email:     "siti@example.com",
		QuotaMode: "DEFAULT",
	})
	if err != nil {
		t.Fatalf("create user default: %v", err)
	}
	if userDefault.QuotaMode != "DEFAULT" {
		t.Errorf("quota_mode = %q, want DEFAULT", userDefault.QuotaMode)
	}
	expected50GB := int64(50) * 1024 * 1024 * 1024
	if userDefault.QuotaBytes != expected50GB {
		t.Errorf("quota_bytes = %d, want %d", userDefault.QuotaBytes, expected50GB)
	}
	if userDefault.QuotaGB == nil || *userDefault.QuotaGB != 50 {
		t.Errorf("quota_gb = %v, want 50", userDefault.QuotaGB)
	}

	// 3. Create user with mode CUSTOM (fractional 1.5 GB)
	onePointFive := 1.5
	userCustom, err := handler.service.CreateUser(ctx, &CreateUserRequest{
		Username:  "budi",
		Email:     "budi@example.com",
		QuotaMode: "CUSTOM",
		QuotaGB:   &onePointFive,
	})
	if err != nil {
		t.Fatalf("create user custom: %v", err)
	}
	if userCustom.QuotaMode != "CUSTOM" {
		t.Errorf("quota_mode = %q, want CUSTOM", userCustom.QuotaMode)
	}
	expected1Point5GB := int64(1.5 * 1024 * 1024 * 1024)
	if userCustom.QuotaBytes != expected1Point5GB {
		t.Errorf("quota_bytes = %d, want %d", userCustom.QuotaBytes, expected1Point5GB)
	}

	// 4. Create user with mode CUSTOM (0 GB)
	zero := 0.0
	userZero, err := handler.service.CreateUser(ctx, &CreateUserRequest{
		Username:  "zero",
		Email:     "zero@example.com",
		QuotaMode: "CUSTOM",
		QuotaGB:   &zero,
	})
	if err != nil {
		t.Fatalf("create user zero: %v", err)
	}
	if userZero.QuotaBytes != 0 {
		t.Errorf("quota_bytes = %d, want 0", userZero.QuotaBytes)
	}

	// 5. Create user with mode UNLIMITED
	userUnlimited, err := handler.service.CreateUser(ctx, &CreateUserRequest{
		Username:  "unlimited_user",
		Email:     "unlimited@example.com",
		QuotaMode: "UNLIMITED",
	})
	if err != nil {
		t.Fatalf("create user unlimited: %v", err)
	}
	if userUnlimited.QuotaBytes != -1 {
		t.Errorf("quota_bytes = %d, want -1", userUnlimited.QuotaBytes)
	}

	// 6. Change system default quota to 100 GB
	hundred := 100
	err = handler.service.UpdateSystemSettings(ctx, &UpdateSettingsRequest{
		DefaultQuota: &QuotaPolicy{Mode: "limited", GB: hundred},
	})
	if err != nil {
		t.Fatalf("update settings to 100: %v", err)
	}

	// Invariant: userDefault snapshot quota must NOT change!
	list, err := handler.service.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	var reloadedSiti *AdminUserResponse
	for _, u := range list.Users {
		if u.ID == userDefault.ID {
			reloadedSiti = &u
			break
		}
	}
	if reloadedSiti == nil {
		t.Fatal("user siti not found")
	}
	if reloadedSiti.QuotaBytes != expected50GB {
		t.Fatalf("siti quota changed after system default changed: got %d, want %d", reloadedSiti.QuotaBytes, expected50GB)
	}

	// 7. Admin updates userDefault to DEFAULT again -> should now snapshot new system default (100 GB)
	updatedSiti, err := handler.service.UpdateUserQuota(ctx, userDefault.ID, &UpdateUserQuotaRequest{
		QuotaMode: "DEFAULT",
	})
	if err != nil {
		t.Fatalf("update siti quota: %v", err)
	}
	expected100GB := int64(100) * 1024 * 1024 * 1024
	if updatedSiti.QuotaBytes != expected100GB {
		t.Errorf("updated siti quota = %d, want %d", updatedSiti.QuotaBytes, expected100GB)
	}

	// 8. Admin updates userCustom to UNLIMITED
	updatedCustom, err := handler.service.UpdateUserQuota(ctx, userCustom.ID, &UpdateUserQuotaRequest{
		QuotaMode: "UNLIMITED",
	})
	if err != nil {
		t.Fatalf("update custom to unlimited: %v", err)
	}
	if updatedCustom.QuotaBytes != -1 || updatedCustom.QuotaMode != "UNLIMITED" {
		t.Errorf("custom to unlimited = %v, want -1", updatedCustom)
	}

	// 9. SUPER_ADMIN quota is locked and always reported as unlimited.
	_, err = db.ExecContext(ctx, `INSERT INTO users(id, username, email, password_hash, role, quota_mode, quota_bytes) VALUES('sa-1', 'admin_boss', 'boss@example.com', 'hash', 'SUPER_ADMIN', 'CUSTOM', 1)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handler.service.UpdateUserQuota(ctx, "sa-1", &UpdateUserQuotaRequest{
		QuotaMode: "CUSTOM",
		QuotaGB:   &onePointFive,
	})
	if err == nil {
		t.Fatal("expected error updating super admin quota, got nil")
	}

	list, err = handler.service.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	var superAdmin *AdminUserResponse
	for _, user := range list.Users {
		if user.ID == "sa-1" {
			superAdmin = &user
			break
		}
	}
	if superAdmin == nil {
		t.Fatal("super admin not found")
	}
	if superAdmin.QuotaMode != "UNLIMITED" || superAdmin.QuotaBytes != -1 {
		t.Fatalf("super admin quota = (%q, %d), want (UNLIMITED, -1)", superAdmin.QuotaMode, superAdmin.QuotaBytes)
	}

}

func TestAdminUsers_ValidationAndHTTPHandlers(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()

	e := echo.New()

	// 1. Duplicate username
	body1, _ := json.Marshal(CreateUserRequest{
		Username: "dupuser",
		Email:    "dup1@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body1))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler.CreateUser(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201", rec.Code)
	}

	// Try again with same username (case-insensitive)
	bodyDup, _ := json.Marshal(CreateUserRequest{
		Username: "DUPUSER",
		Email:    "dup2@example.com",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(bodyDup))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err := handler.CreateUser(c)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if echoErr, ok := err.(*echo.HTTPError); ok {
		if echoErr.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", echoErr.Code)
		}
	} else {
		t.Fatalf("expected echo.HTTPError, got %v", err)
	}

	// 2. List users HTTP
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := handler.GetUsers(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var listResp AdminUsersListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Users) < 1 {
		t.Fatalf("got %d users, want at least 1", len(listResp.Users))
	}

	// 3. Delete user HTTP without exact email confirmation -> fails
	createdID := listResp.Users[0].ID
	createdEmail := listResp.Users[0].Email
	bodyWrong, _ := json.Marshal(DeleteUserRequest{Email: "wrong@example.com"})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+createdID, bytes.NewReader(bodyWrong))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(createdID)
	c.Set("user_id", "admin-super")
	err = handler.DeleteUser(c)
	if err == nil {
		t.Fatal("expected error with mismatched email, got nil")
	}

	// Delete user with exact email match -> succeeds
	bodyRight, _ := json.Marshal(DeleteUserRequest{Email: createdEmail})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+createdID, bytes.NewReader(bodyRight))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(createdID)
	c.Set("user_id", "admin-super")
	if err := handler.DeleteUser(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}

	// 4. Cannot delete self
	bodySelf, _ := json.Marshal(DeleteUserRequest{Email: "self@example.com"})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/self-user", bytes.NewReader(bodySelf))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("self-user")
	c.Set("user_id", "self-user")
	err = handler.DeleteUser(c)
	if err == nil {
		t.Fatal("expected error deleting self, got nil")
	}
	if echoErr, ok := err.(*echo.HTTPError); ok {
		if echoErr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", echoErr.Code)
		}
	}
}

func TestAdminUsers_DeleteUser_CompleteFlow(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create target regular user
	user, err := handler.service.CreateUser(ctx, &CreateUserRequest{
		Username: "charlie",
		Email:    "charlie@company.com",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 2. Insert items for charlie in my_storage_items (active and in trash)
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key)
		VALUES ('item-active-1', ?, 'active.txt', 'file', 100, 'disk-key-exclusive'),
		       ('item-trash-1', ?, 'trash.txt', 'file', 200, 'disk-key-trash')
	`, user.ID, user.ID)
	if err != nil {
		t.Fatalf("insert personal items: %v", err)
	}
	// Mark item-trash-1 as trash
	_, err = db.ExecContext(ctx, `UPDATE my_storage_items SET deleted_at = CURRENT_TIMESTAMP WHERE id = 'item-trash-1'`)
	if err != nil {
		t.Fatalf("mark trash: %v", err)
	}

	// 3. Insert organization item created by charlie
	_, err = db.ExecContext(ctx, `
		INSERT INTO organization_items (id, created_by, name, kind, size_bytes)
		VALUES ('org-item-1', ?, 'team-project', 'folder', 0)
	`, user.ID)
	if err != nil {
		t.Fatalf("insert org item: %v", err)
	}

	// 4. Verification: attempt delete with wrong email (case mismatch)
	err = handler.service.DeleteUser(ctx, "admin-super", user.ID, &DeleteUserRequest{Email: "CHARLIE@company.com"})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected exact email mismatch error, got %v", err)
	}

	// 5. Delete with exact email
	err = handler.service.DeleteUser(ctx, "admin-super", user.ID, &DeleteUserRequest{Email: "charlie@company.com"})
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	// 6. Verify all personal storage items (both active and trash) are permanently removed
	var personalCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM my_storage_items WHERE owner_id = ?`, user.ID).Scan(&personalCount)
	if err != nil {
		t.Fatalf("query personal count: %v", err)
	}
	if personalCount != 0 {
		t.Fatalf("personal storage items remaining: %d, want 0", personalCount)
	}

	// 7. Verify user is tombstoned
	var tombstoneUser struct {
		Username  string
		Email     string
		DeletedAt *string
	}
	err = db.QueryRowContext(ctx, `SELECT username, email, deleted_at FROM users WHERE id = ?`, user.ID).Scan(&tombstoneUser.Username, &tombstoneUser.Email, &tombstoneUser.DeletedAt)
	if err != nil {
		t.Fatalf("query tombstoned user: %v", err)
	}
	if tombstoneUser.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set, got nil")
	}
	if !strings.HasPrefix(tombstoneUser.Username, "deleted_user_") {
		t.Errorf("username = %s, want prefix deleted_user_", tombstoneUser.Username)
	}
	if !strings.HasPrefix(tombstoneUser.Email, "deleted_") {
		t.Errorf("email = %s, want prefix deleted_", tombstoneUser.Email)
	}

	// 8. Verify user is no longer listed in active users
	list, err := handler.service.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	for _, u := range list.Users {
		if u.ID == user.ID {
			t.Fatalf("tombstoned user still appears in ListUsers: %v", u)
		}
	}

	// 9. Verify organization item still exists and creator attribution displays "deleted user"
	var orgCreator string
	err = db.QueryRowContext(ctx, `
		SELECT CASE
			WHEN u.id IS NULL OR u.deleted_at IS NOT NULL THEN 'deleted user'
			ELSE u.username
		END AS created_by
		FROM organization_items o
		LEFT JOIN users u ON u.id = o.created_by
		WHERE o.id = 'org-item-1'
	`).Scan(&orgCreator)
	if err != nil {
		t.Fatalf("query org creator: %v", err)
	}
	if orgCreator != "deleted user" {
		t.Fatalf("organization creator attribution = %s, want 'deleted user'", orgCreator)
	}
}

func TestAdminUsers_ResetPasswordIssuesTokenAndDisablesPassword(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()
	created, err := handler.service.CreateUser(ctx, &CreateUserRequest{Username: "pending", Email: "pending@example.com", QuotaMode: "DEFAULT"})
	if err != nil { t.Fatal(err) }
	var passwordHash, status string
	if err := db.QueryRowContext(ctx, `SELECT password_hash, status FROM users WHERE id = ?`, created.ID).Scan(&passwordHash, &status); err != nil { t.Fatal(err) }
	if passwordHash != "" || status != "PENDING_ACTIVATION" { t.Fatalf("pending credentials/status = %q/%q", passwordHash, status) }
	if created.InvitationToken == "" || created.InvitationURL != "/set-password#token="+created.InvitationToken { t.Fatal("missing invitation response") }
	var version int
	if err := db.QueryRowContext(ctx, `SELECT token_version FROM users WHERE id = ?`, created.ID).Scan(&version); err != nil { t.Fatal(err) }
	token, expires, err := handler.service.ResetUserPassword(ctx, "super-admin-id", created.ID)
	if err != nil { t.Fatal(err) }
	if token == "" || expires.Sub(time.Now()) < 29*time.Minute { t.Fatalf("invalid reset expiration: %v", expires) }
	if err := db.QueryRowContext(ctx, `SELECT password_hash, token_version FROM users WHERE id = ?`, created.ID).Scan(&passwordHash, &version); err != nil { t.Fatal(err) }
	if passwordHash != "" || version != 1 { t.Fatalf("reset hash/version = %q/%d", passwordHash, version) }
	var tokenType string
	if err := db.QueryRowContext(ctx, `SELECT token_type FROM auth_tokens WHERE user_id = ? ORDER BY rowid DESC LIMIT 1`, created.ID).Scan(&tokenType); err != nil { t.Fatal(err) }
	if tokenType != "PASSWORD_RESET" { t.Fatalf("token type = %q", tokenType) }
}

