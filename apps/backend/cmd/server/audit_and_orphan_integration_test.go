package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/jobs"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/admin"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testS3Driver struct {
	storage.StorageDriver
	s3Objects   map[string]bool
	deletedKeys []string
}

func (d *testS3Driver) ListObjects(_ context.Context, _ string, prefix string) ([]string, error) {
	keys := make([]string, 0, len(d.s3Objects))
	for k := range d.s3Objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (d *testS3Driver) DeleteObject(_ context.Context, _, key string) error {
	delete(d.s3Objects, key)
	d.deletedKeys = append(d.deletedKeys, key)
	return nil
}

func (d *testS3Driver) ReadObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}
func (d *testS3Driver) WriteObject(context.Context, string, string, io.Reader, int64) error {
	return nil
}
func (d *testS3Driver) HeadObject(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (d *testS3Driver) PresignUpload(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (d *testS3Driver) PresignDownload(context.Context, string, string, string, time.Duration) (string, error) {
	return "", nil
}

func TestStartupOrphanBlobReconciliation_LocalAndS3(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "db"), 0o755))

	db, err := database.Connect(dataDir)
	require.NoError(t, err)
	defer db.Close()

	localNodePath := filepath.Join(dataDir, "local_storage")
	require.NoError(t, os.MkdirAll(localNodePath, 0o755))

	instanceID, err := storage.InstanceID(ctx, db)
	require.NoError(t, err)

	localBlobPrefix := filepath.Join(localNodePath, "cubycloud", instanceID, "local-node-1", "blobs")
	require.NoError(t, os.MkdirAll(localBlobPrefix, 0o755))

	// File outside owned namespace (must NEVER be deleted)
	outsidePath := filepath.Join(localNodePath, "outside.bin")
	require.NoError(t, os.WriteFile(outsidePath, []byte("outside content"), 0o600))
	oldTime := time.Now().Add(-100 * time.Hour)
	require.NoError(t, os.Chtimes(outsidePath, oldTime, oldTime))

	// Create test files within local owned namespace:
	// 1. active file (referenced in my_storage_items)
	// 2. trashed file (referenced in my_storage_items with deleted_at)
	// 3. genuine orphan file (no reference, old modtime)
	for _, name := range []string{"active.bin", "trash.bin", "orphan.bin"} {
		path := filepath.Join(localBlobPrefix, name)
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o600))
		require.NoError(t, os.Chtimes(path, oldTime, oldTime))
	}
	// Register storage nodes in SQLite
	_, err = db.ExecContext(ctx, `INSERT INTO storage_nodes(id, name, type, path, capacity_bytes, enabled)
		VALUES('local-node-1', 'Local Node', 'LOCAL', ?, 1073741824, 1)`, localNodePath)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO storage_nodes(id, name, type, bucket, capacity_bytes, enabled)
		VALUES('s3-node-1', 'S3 Bucket', 'S3', 'test-bucket', 1073741824, 1)`)
	require.NoError(t, err)

	// Create user
	_, err = db.ExecContext(ctx, `INSERT INTO users(id, username, email, password_hash, role)
		VALUES('user-1', 'tester', 'tester@test.local', 'hash', 'REGULAR')`)
	require.NoError(t, err)

	activeKey := storage.BuildBlobKey(instanceID, "local-node-1", "active.bin")
	trashKey := storage.BuildBlobKey(instanceID, "local-node-1", "trash.bin")
	s3RefKey := storage.BuildBlobKey(instanceID, "s3-node-1", "s3-referenced.bin")
	s3PendingKey := storage.BuildBlobKey(instanceID, "s3-node-1", "s3-pending-active")
	s3Orphan1Key := storage.BuildBlobKey(instanceID, "s3-node-1", "s3-orphan-1.bin")
	s3Orphan2Key := storage.BuildBlobKey(instanceID, "s3-node-1", "s3-orphan-2.bin")

	// Reference active and trashed files in SQLite
	_, err = db.ExecContext(ctx, `INSERT INTO my_storage_items(id, owner_id, name, kind, disk_key, storage_node_id, deleted_at)
		VALUES('item-active', 'user-1', 'active.bin', 'file', ?, 'local-node-1', NULL)`, activeKey)
	require.NoError(t, err)

	trashTime := time.Now().Add(-2 * time.Hour)
	_, err = db.ExecContext(ctx, `INSERT INTO my_storage_items(id, owner_id, name, kind, disk_key, storage_node_id, deleted_at)
		VALUES('item-trash', 'user-1', 'trash.bin', 'file', ?, 'local-node-1', ?)`, trashKey, trashTime)
	require.NoError(t, err)

	// Active and unexpired pending cloud upload for S3
	_, err = db.ExecContext(ctx, `INSERT INTO pending_cloud_uploads(id, owner_id, storage_node_id, filename, size_bytes, expires_at)
		VALUES('s3-pending-active', 'user-1', 's3-node-1', 'pending.bin', 100, datetime('now', '+2 hours'))`)
	require.NoError(t, err)

	// S3 referenced file in organization
	_, err = db.ExecContext(ctx, `INSERT INTO organization_items(id, created_by, name, kind, disk_key, storage_node_id)
		VALUES('org-s3-item', 'user-1', 'org_s3.bin', 'file', ?, 's3-node-1')`, s3RefKey)
	require.NoError(t, err)

	// Setup S3 test driver with objects:
	// - s3-referenced.bin (referenced)
	// - s3-pending-active (pending upload intent)
	// - s3-orphan-1.bin (genuine orphan)
	// - s3-orphan-2.bin (genuine orphan)
	// - outside-bucket-file.bin (outside owned namespace, must NEVER be touched)
	// - .system_metadata (ignored system prefix)
	s3Objects := map[string]bool{
		s3RefKey:                 true,
		s3PendingKey:             true,
		s3Orphan1Key:             true,
		s3Orphan2Key:             true,
		"unrelated/outside.bin":  true,
		".system_metadata":       true,
	}

	// Setup S3 test driver with objects:
	// - s3-referenced.bin (referenced)
	// - s3-pending-active (pending upload intent)
	// - s3-orphan-1.bin (genuine orphan)
	// - s3-orphan-2.bin (genuine orphan)
	// - .system_metadata (ignored system prefix)
	driver := &testS3Driver{
		StorageDriver: storage.NewLocalDriver(dataDir),
		s3Objects:     s3Objects,
	}


	// Observation 1: default dryRun mode audits candidates
	report1, err := storage.ReconcileOrphanBlobs(ctx, db, driver, 1*time.Hour, true)
	require.NoError(t, err)
	assert.Empty(t, report1.Errors)
	assert.Equal(t, 0, report1.LocalDeleted, "dry run should not delete")
	assert.Equal(t, 0, report1.S3Deleted, "dry run should not delete")

	// Advance candidate observation time past grace period
	_, err = db.ExecContext(ctx, `UPDATE orphan_candidates SET first_seen_at=datetime('now', '-2 hours')`)
	require.NoError(t, err)

	// Observation 2: deletion enabled (dryRun=false)
	report2, err := storage.ReconcileOrphanBlobs(ctx, db, driver, 1*time.Hour, false)
	require.NoError(t, err)
	assert.Empty(t, report2.Errors)

	// Verify local results
	assert.Equal(t, 1, report2.LocalDeleted, "only orphan.bin should be deleted locally")
	assert.NoFileExists(t, filepath.Join(localBlobPrefix, "orphan.bin"))
	assert.FileExists(t, filepath.Join(localBlobPrefix, "active.bin"), "active file must be preserved")
	assert.FileExists(t, filepath.Join(localBlobPrefix, "trash.bin"), "trashed file must be preserved until purge")
	assert.FileExists(t, outsidePath, "unnamespaced outside file must NEVER be touched")

	// Verify S3 results
	assert.Equal(t, 2, report2.S3Deleted, "both s3-orphan-1 and s3-orphan-2 should be deleted")
	assert.Contains(t, driver.deletedKeys, s3Orphan1Key)
	assert.Contains(t, driver.deletedKeys, s3Orphan2Key)
	assert.True(t, driver.s3Objects[s3RefKey], "referenced S3 object must be preserved")
	assert.True(t, driver.s3Objects[s3PendingKey], "active pending S3 upload must be preserved")
	assert.True(t, driver.s3Objects["unrelated/outside.bin"], "outside object must NEVER be touched")
	assert.True(t, driver.s3Objects[".system_metadata"], "system dot-files must be ignored")
}

func TestAuditLogMiddleware_MutationsAndRedactionAndRetention(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "db"), 0o755))

	db, err := database.Connect(dataDir)
	require.NoError(t, err)
	defer db.Close()

	// Seed settings with 7-day retention
	_, err = db.ExecContext(ctx, `INSERT OR REPLACE INTO settings (key, value) VALUES ('audit_retention_days', '7')`)
	require.NoError(t, err)

	e := echo.New()
	e.Use(middleware.AuditLog(db))

	// Dummy endpoints to simulate various handlers
	e.POST("/api/v1/auth/login", func(c echo.Context) error {
		var body map[string]any
		_ = c.Bind(&body)
		if body["username"] == "fail" {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
		c.Set("user_id", "admin-1")
		return c.JSON(http.StatusOK, map[string]string{"token": "secret-jwt-token"})
	})

	e.PUT("/api/v1/admin/settings", func(c echo.Context) error {
		c.Set("user_id", "admin-1")
		return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
	})

	e.DELETE("/api/v1/drive/items/:id", func(c echo.Context) error {
		id := c.Param("id")
		if id == "not-found" {
			return echo.NewHTTPError(http.StatusNotFound, "item not found")
		}
		c.Set("user_id", "user-123")
		return c.NoContent(http.StatusNoContent)
	})

	e.GET("/api/v1/drive/items", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []string{"file1", "file2"})
	})

	// 1. Successful POST with sensitive credentials
	{
		payload := `{"username":"admin","password":"SuperSecretPassword123","emergency_key":"RecoveryKeyABC"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 2. Failed POST (401 Unauthorized)
	{
		payload := `{"username":"fail","password":"WrongPassword"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	// 3. Successful PUT
	{
		payload := `{"audit_retention_days":30,"secret_key":"SensitiveCloudSecret"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// 4. Successful DELETE
	{
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/drive/items/file-456", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}

	// 5. Failed DELETE (404 Not Found)
	{
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/drive/items/not-found", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	}

	// 6. GET request (MUST NOT be audited)
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/items", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// Verify Audit Log records in SQLite
	var logs []database.AuditLog
	err = db.NewSelect().Model(&logs).Order("created_at ASC").Scan(ctx)
	require.NoError(t, err)

	// Expect exactly 5 mutation rows (GET is omitted)
	require.Len(t, logs, 5, "must contain exactly 5 audited mutations and 0 GET requests")

	// Verify no GET was logged
	getCount, err := db.NewSelect().Table("audit_logs").Where("method = 'GET'").Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, getCount, "GET requests must not be audited")

	// Verify Redaction on sensitive fields
	for _, row := range logs {
		if row.Details != nil {
			details := *row.Details
			assert.NotContains(t, details, "SuperSecretPassword123", "password must be redacted")
			assert.NotContains(t, details, "WrongPassword", "password must be redacted")
			assert.NotContains(t, details, "RecoveryKeyABC", "emergency/recovery key must be redacted")
			assert.NotContains(t, details, "SensitiveCloudSecret", "secret key must be redacted")
			if strings.Contains(details, "password") {
				assert.Contains(t, details, "[REDACTED]")
			}
		}
	}

	// Verify Failed mutation captures status code & error message
	var failedLogin database.AuditLog
	err = db.NewSelect().Model(&failedLogin).Where("status_code = 401").Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, 401, failedLogin.StatusCode)
	assert.NotNil(t, failedLogin.ErrorMessage)
	assert.Contains(t, *failedLogin.ErrorMessage, "401")

	var failedDelete database.AuditLog
	err = db.NewSelect().Model(&failedDelete).Where("status_code = 404").Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, 404, failedDelete.StatusCode)
	assert.NotNil(t, failedDelete.ErrorMessage)
	assert.Contains(t, *failedDelete.ErrorMessage, "404")

	// Test Admin API endpoint GET /api/v1/admin/audit-logs
	adminHandler := admin.NewHandler(db, storage.NewLocalDriver(dataDir), dataDir, filepath.Join(dataDir, "db", "cubycloud.db"))
	eAdmin := echo.New()
	eAdmin.GET("/api/v1/admin/audit-logs", adminHandler.GetAuditLogs)

	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?limit=10", nil)
	recAdmin := httptest.NewRecorder()
	eAdmin.ServeHTTP(recAdmin, reqAdmin)
	assert.Equal(t, http.StatusOK, recAdmin.Code)

	var retrievedLogs []map[string]any
	require.NoError(t, json.Unmarshal(recAdmin.Body.Bytes(), &retrievedLogs))
	assert.Len(t, retrievedLogs, 5)

	// Test Retention Purge:
	// Insert an older audit log that exceeds 7 days (e.g., 10 days old)
	oldTimestamp := time.Now().UTC().AddDate(0, 0, -10)
	_, err = db.ExecContext(ctx, `INSERT INTO audit_logs (id, method, path, status_code, action, created_at)
		VALUES ('old-log-id', 'DELETE', '/old/path', 200, 'DELETE /old/path', ?)`, oldTimestamp)
	require.NoError(t, err)

	// Run retention purge with 7-day setting
	purged, err := jobs.PurgeExpiredAuditLogs(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, int64(1), purged, "should purge exactly 1 old audit log older than 7 days")

	oldLogExists, err := db.NewSelect().Table("audit_logs").Where("id = 'old-log-id'").Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, oldLogExists, "old log must be removed")

	// Ensure the 5 recent logs remain intact
	remainingCount, err := db.NewSelect().Table("audit_logs").Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, remainingCount)
}
