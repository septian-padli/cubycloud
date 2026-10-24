package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/labstack/echo/v4"
)

func TestAdminStorage_GetStorageStats(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()

	const gb = int64(1024 * 1024 * 1024)

	// 1. Insert 2 storage nodes (100 GB and 200 GB)
	_, err := db.ExecContext(ctx, `
		INSERT INTO storage_nodes (id, name, type, capacity_bytes, enabled, created_at)
		VALUES
			('node-1', 'Local Disk 1', 'LOCAL', ?, 1, CURRENT_TIMESTAMP),
			('node-2', 'Local Disk 2', 'LOCAL', ?, 1, CURRENT_TIMESTAMP)
	`, 100*gb, 200*gb)
	if err != nil {
		t.Fatalf("insert storage nodes: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, username, email, password_hash, role)
		VALUES
			('user-1', 'u1', 'u1@example.com', 'h1', 'REGULAR'),
			('user-2', 'u2', 'u2@example.com', 'h2', 'REGULAR')
	`)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	// 2. Insert items in my_storage_items:
	// - user1: file dk1 (10 GB)
	// - user2: file dk1 (10 GB, same disk_key, should be deduplicated)
	// - user2: file dk2 (40 GB)
	// - user1: folder dk3 (30 GB, kind = folder, should be ignored)
	// - user1: file with null disk_key (should be ignored)
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, modified_at, deleted_at)
		VALUES
			('item-1', 'user-1', 'doc1.pdf', 'file', ?, 'disk-key-1', CURRENT_TIMESTAMP, NULL),
			('item-2', 'user-2', 'doc1_copy.pdf', 'file', ?, 'disk-key-1', CURRENT_TIMESTAMP, NULL),
			('item-3', 'user-2', 'video.mp4', 'file', ?, 'disk-key-2', CURRENT_TIMESTAMP, NULL),
			('item-4', 'user-1', 'folder1', 'folder', ?, 'disk-key-folder', CURRENT_TIMESTAMP, NULL),
			('item-5', 'user-1', 'file_no_disk.txt', 'file', ?, NULL, CURRENT_TIMESTAMP, NULL),
			('item-6', 'user-1', 'photo.jpg', 'file', ?, 'disk-key-6', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, 10*gb, 10*gb, 40*gb, 30*gb, 5*gb, 5*gb)
	if err != nil {
		t.Fatalf("insert storage items: %v", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO organization_items (id, created_by, name, kind, size_bytes, disk_key, modified_at, deleted_at)
		VALUES ('org-item-1', 'user-1', 'org-file.bin', 'file', ?, 'org-disk-key', CURRENT_TIMESTAMP, NULL)
	`, 3*gb)
	if err != nil {
		t.Fatalf("insert organization storage item: %v", err)
	}

	// 3. Test service GetAdminStorageStats
	stats, err := handler.service.GetAdminStorageStats(ctx)
	if err != nil {
		t.Fatalf("GetAdminStorageStats: %v", err)
	}

	expectedCap := 300 * gb
	expectedUsed := 58 * gb // 10 GB (dk1) + 40 GB (dk2) + 5 GB (dk6 trash) + 3 GB organization file
	expectedFree := 242 * gb

	if stats.TotalCapacityBytes != expectedCap {
		t.Errorf("TotalCapacityBytes = %d, want %d", stats.TotalCapacityBytes, expectedCap)
	}
	if stats.TotalCapacityGB != 300.0 {
		t.Errorf("TotalCapacityGB = %v, want 300.0", stats.TotalCapacityGB)
	}
	if stats.TotalUsedBytes != expectedUsed {
		t.Errorf("TotalUsedBytes = %d, want %d", stats.TotalUsedBytes, expectedUsed)
	}
	if stats.TotalUsedGB != 58.0 {
		t.Errorf("TotalUsedGB = %v, want 58.0", stats.TotalUsedGB)
	}
	if stats.FreeBytes != expectedFree {
		t.Errorf("FreeBytes = %d, want %d", stats.FreeBytes, expectedFree)
	}
	if stats.FreeGB != 242.0 {
		t.Errorf("FreeGB = %v, want 242.0", stats.FreeGB)
	}
	if stats.NodesCount != 2 {
		t.Errorf("NodesCount = %d, want 2", stats.NodesCount)
	}

	// Verify categories
	catMap := make(map[string]AdminStorageCategoryUsage)
	for _, c := range stats.Categories {
		catMap[c.ID] = c
	}

	if cat, ok := catMap["video"]; !ok || cat.Bytes != 40*gb || cat.UsedGB != 40.0 {
		t.Errorf("video category = %+v, want 40 GB", cat)
	}
	if cat, ok := catMap["document"]; !ok || cat.Bytes != 10*gb || cat.UsedGB != 10.0 {
		t.Errorf("document category = %+v, want 10 GB", cat)
	}
	if cat, ok := catMap["image"]; !ok || cat.Bytes != 0 || cat.UsedGB != 0.0 {
		t.Errorf("image category = %+v, want 0 GB (photo.jpg is trashed)", cat)
	}
	if cat, ok := catMap["trash"]; !ok || cat.Bytes != 5*gb || cat.UsedGB != 5.0 {
		t.Errorf("trash category = %+v, want 5 GB", cat)
	}
	// 4. Test HTTP handler GetStorageStats
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetStorageStats(c); err != nil {
		t.Fatalf("handler.GetStorageStats: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var httpResp AdminStorageStatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &httpResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if httpResp.NodesCount != 2 || httpResp.TotalCapacityBytes != expectedCap || len(httpResp.Categories) == 0 {
		t.Errorf("unexpected http response: %+v", httpResp)
	}
}

func TestAdminStorage_GetTopStorageUsers(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()

	const gb = int64(1024 * 1024 * 1024)

	// 1. Insert 4 users:
	// - user-admin: SUPER_ADMIN, quota_bytes = -1
	// - user-alpha: REGULAR, quota_mode = CUSTOM, quota_bytes = 100 GB
	// - user-beta:  REGULAR, quota_mode = DEFAULT, quota_bytes = 50 GB
	// - user-gamma: REGULAR, quota_mode = DEFAULT, quota_bytes = 50 GB
	// - user-deleted: REGULAR, deleted_at not null (should be ignored)
	now := time.Now()
	users := []database.User{
		{
			ID:           "user-admin",
			Username:     "superadmin",
			Email:        "admin@example.com",
			PasswordHash: "hash",
			Role:         "SUPER_ADMIN",
			QuotaMode:    "DEFAULT", // should be normalized to UNLIMITED
			QuotaBytes:   50 * gb,
			CreatedAt:    now.Add(-4 * time.Hour),
			UpdatedAt:    now,
		},
		{
			ID:           "user-alpha",
			Username:     "alpha",
			Email:        "alpha@example.com",
			PasswordHash: "hash",
			Role:         "REGULAR",
			QuotaMode:    "CUSTOM",
			QuotaBytes:   100 * gb,
			CreatedAt:    now.Add(-3 * time.Hour),
			UpdatedAt:    now,
		},
		{
			ID:           "user-beta",
			Username:     "beta",
			Email:        "beta@example.com",
			PasswordHash: "hash",
			Role:         "REGULAR",
			QuotaMode:    "DEFAULT",
			QuotaBytes:   50 * gb,
			CreatedAt:    now.Add(-2 * time.Hour),
			UpdatedAt:    now,
		},
		{
			ID:           "user-gamma",
			Username:     "gamma",
			Email:        "gamma@example.com",
			PasswordHash: "hash",
			Role:         "REGULAR",
			QuotaMode:    "DEFAULT",
			QuotaBytes:   50 * gb,
			CreatedAt:    now.Add(-1 * time.Hour),
			UpdatedAt:    now,
		},
	}
	for _, u := range users {
		if _, err := db.NewInsert().Model(&u).Exec(ctx); err != nil {
			t.Fatalf("insert user %s: %v", u.Username, err)
		}
	}

	// Insert deleted user
	deletedAt := now
	deletedUser := database.User{
		ID:           "user-deleted",
		Username:     "deleted",
		Email:        "deleted@example.com",
		PasswordHash: "hash",
		Role:         "REGULAR",
		CreatedAt:    now.Add(-5 * time.Hour),
		UpdatedAt:    now,
		DeletedAt:    &deletedAt,
	}
	if _, err := db.NewInsert().Model(&deletedUser).Exec(ctx); err != nil {
		t.Fatalf("insert deleted user: %v", err)
	}

	// 2. Insert storage items to give users different usage:
	// - user-alpha: 50 GB (item 1: 30 GB, item 2: 20 GB, duplicate item: 20 GB on same disk_key)
	// - user-beta: 25 GB
	// - user-admin: 10 GB
	// - user-gamma: 0 GB
	// - user-deleted: 100 GB (should be ignored since user is deleted)
	_, err := db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, modified_at)
		VALUES
			('item-a1', 'user-alpha', 'a1.zip', 'file', ?, 'dk-a1', CURRENT_TIMESTAMP),
			('item-a2', 'user-alpha', 'a2.zip', 'file', ?, 'dk-a2', CURRENT_TIMESTAMP),
			('item-a2-dup', 'user-alpha', 'a2_copy.zip', 'file', ?, 'dk-a2', CURRENT_TIMESTAMP),
			('item-b1', 'user-beta', 'b1.mp4', 'file', ?, 'dk-b1', CURRENT_TIMESTAMP),
			('item-adm', 'user-admin', 'adm.log', 'file', ?, 'dk-adm', CURRENT_TIMESTAMP),
			('item-del', 'user-deleted', 'huge.iso', 'file', ?, 'dk-del', CURRENT_TIMESTAMP)
	`, 30*gb, 20*gb, 20*gb, 25*gb, 10*gb, 100*gb)
	if err != nil {
		t.Fatalf("insert my_storage_items: %v", err)
	}

	// 3. Test pagination Page 1: limit=2, offset=0
	// Expected order:
	// 1. user-alpha (50 GB)
	// 2. user-beta  (25 GB)
	page1, err := handler.service.GetAdminTopStorageUsers(ctx, 2, 0)
	if err != nil {
		t.Fatalf("GetAdminTopStorageUsers page 1: %v", err)
	}
	if page1.Total != 4 {
		t.Errorf("page 1 Total = %d, want 4", page1.Total)
	}
	if !page1.HasMore {
		t.Errorf("page 1 HasMore = false, want true")
	}
	if page1.NextOffset != 2 {
		t.Errorf("page 1 NextOffset = %d, want 2", page1.NextOffset)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("page 1 items length = %d, want 2", len(page1.Items))
	}
	if page1.Items[0].ID != "user-alpha" || page1.Items[0].UsedBytes != 50*gb {
		t.Errorf("page 1 item[0] = %+v, want user-alpha with 50 GB", page1.Items[0])
	}
	if page1.Items[1].ID != "user-beta" || page1.Items[1].UsedBytes != 25*gb {
		t.Errorf("page 1 item[1] = %+v, want user-beta with 25 GB", page1.Items[1])
	}

	// 4. Test pagination Page 2: limit=2, offset=2
	// Expected order:
	// 3. user-admin (10 GB) - SUPER_ADMIN normalized to UNLIMITED, -1, nil
	// 4. user-gamma (0 GB)
	page2, err := handler.service.GetAdminTopStorageUsers(ctx, 2, 2)
	if err != nil {
		t.Fatalf("GetAdminTopStorageUsers page 2: %v", err)
	}
	if page2.Total != 4 {
		t.Errorf("page 2 Total = %d, want 4", page2.Total)
	}
	if page2.HasMore {
		t.Errorf("page 2 HasMore = true, want false")
	}
	if page2.NextOffset != 4 {
		t.Errorf("page 2 NextOffset = %d, want 4", page2.NextOffset)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("page 2 items length = %d, want 2", len(page2.Items))
	}
	if page2.Items[0].ID != "user-admin" || page2.Items[0].UsedBytes != 10*gb {
		t.Errorf("page 2 item[0] = %+v, want user-admin with 10 GB", page2.Items[0])
	}
	// Check SUPER_ADMIN normalization
	if page2.Items[0].QuotaMode != "UNLIMITED" {
		t.Errorf("super admin QuotaMode = %q, want UNLIMITED", page2.Items[0].QuotaMode)
	}
	if page2.Items[0].QuotaBytes != -1 {
		t.Errorf("super admin QuotaBytes = %d, want -1", page2.Items[0].QuotaBytes)
	}
	if page2.Items[0].QuotaGB != nil {
		t.Errorf("super admin QuotaGB = %v, want nil", page2.Items[0].QuotaGB)
	}

	if page2.Items[1].ID != "user-gamma" || page2.Items[1].UsedBytes != 0 {
		t.Errorf("page 2 item[1] = %+v, want user-gamma with 0 bytes", page2.Items[1])
	}

	// 5. Test beyond total offset: limit=2, offset=4
	page3, err := handler.service.GetAdminTopStorageUsers(ctx, 2, 4)
	if err != nil {
		t.Fatalf("GetAdminTopStorageUsers page 3: %v", err)
	}
	if len(page3.Items) != 0 {
		t.Errorf("page 3 items = %d, want 0", len(page3.Items))
	}
	if page3.HasMore {
		t.Errorf("page 3 HasMore = true, want false")
	}

	// 6. Test HTTP handler GetTopStorageUsers
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage/top-users?limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetTopStorageUsers(c); err != nil {
		t.Fatalf("handler.GetTopStorageUsers: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var httpResp AdminTopStorageUsersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &httpResp); err != nil {
		t.Fatalf("unmarshal top users response: %v", err)
	}
	if httpResp.Total != 4 || len(httpResp.Items) != 2 {
		t.Errorf("unexpected http response: %+v", httpResp)
	}
}

func TestAdminStorage_RouteAccessControl(t *testing.T) {
	db, _, dbPath := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()

	// Insert a super admin and a regular user in DB
	adminUser := &database.User{
		ID:           "admin-super-id",
		Username:     "admin_super",
		Email:        "admin_super@example.com",
		PasswordHash: "hash",
		Role:         "SUPER_ADMIN",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := db.NewInsert().Model(adminUser).Exec(ctx); err != nil {
		t.Fatalf("insert super admin: %v", err)
	}

	regularUser := &database.User{
		ID:           "regular-id",
		Username:     "regular_user",
		Email:        "regular@example.com",
		PasswordHash: "hash",
		Role:         "REGULAR",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := db.NewInsert().Model(regularUser).Exec(ctx); err != nil {
		t.Fatalf("insert regular user: %v", err)
	}

	// Setup Echo with RegisterRoutes
	e := echo.New()
	driver := &mockStorageDriver{}
	dataDir := t.TempDir()

	jwtAuth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// JWT middleware extracts user_id and user_role from claims/header
			authHeader := c.Request().Header.Get("X-Test-User-ID")
			if authHeader != "" {
				c.Set("user_id", authHeader)
			}
			roleHeader := c.Request().Header.Get("X-Test-User-Role")
			if roleHeader != "" {
				c.Set("user_role", roleHeader)
			}
			return next(c)
		}
	}
	superAdminGuard := middleware.SuperAdminGuard(db)

	RegisterRoutes(e, db, driver, dataDir, dbPath, jwtAuth, superAdminGuard)

	endpoints := []string{
		"/api/v1/admin/storage/stats",
		"/api/v1/admin/storage/top-users",
	}

	for _, ep := range endpoints {
		// 1. Unauthenticated -> 401 Unauthorized
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("[%s] unauthenticated status = %d, want 401", ep, rec.Code)
		}

		// 2. Authenticated as REGULAR user -> 403 Forbidden
		req = httptest.NewRequest(http.MethodGet, ep, nil)
		req.Header.Set("X-Test-User-ID", regularUser.ID)
		req.Header.Set("X-Test-User-Role", regularUser.Role)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("[%s] regular user status = %d, want 403", ep, rec.Code)
		}

		// 3. Authenticated as SUPER_ADMIN -> 200 OK
		req = httptest.NewRequest(http.MethodGet, ep, nil)
		req.Header.Set("X-Test-User-ID", adminUser.ID)
		req.Header.Set("X-Test-User-Role", adminUser.Role)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("[%s] super admin status = %d, want 200, body = %s", ep, rec.Code, rec.Body.String())
		}
	}
}
