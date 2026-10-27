package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

type mockStorageDriver struct{}

func (*mockStorageDriver) WriteLocal(_ context.Context, _, _ string, _ io.Reader) error {
	return nil
}

func (*mockStorageDriver) ReadLocal(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (*mockStorageDriver) DeleteLocal(_ context.Context, _, _ string) error {
	return nil
}

func (*mockStorageDriver) StatLocal(context.Context, string) (int64, int64, error) {
	return 10000000000, 5000000000, nil
}

func (*mockStorageDriver) GeneratePresignedUploadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (*mockStorageDriver) GeneratePresignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (*mockStorageDriver) ValidateCloudCredentials(context.Context, string, string, string, string, string) error {
	return nil
}

func setupTestAdmin(t *testing.T) (*bun.DB, *Handler, string) {
	dataDir := t.TempDir()
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "cubycloud.db")

	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	driver := &mockStorageDriver{}
	handler := NewHandler(db, driver, dataDir, dbPath)

	return db, handler, dbPath
}

func TestGetAndSetSettings(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Initial settings
	settings, err := handler.service.GetSystemSettings(ctx)
	if err != nil {
		t.Fatalf("failed to get initial settings: %v", err)
	}
	if settings.DefaultQuota.Mode != "unlimited" {
		t.Errorf("expected default quota unlimited, got %s", settings.DefaultQuota.Mode)
	}
	if settings.AuditRetentionDays != 90 {
		t.Errorf("expected audit retention 90, got %d", settings.AuditRetentionDays)
	}
	if settings.WorkerConcurrency != 2 {
		t.Errorf("expected worker concurrency 2, got %d", settings.WorkerConcurrency)
	}

	// 2. Update settings valid
	retention := 30
	concurrency := 4
	updateReq := &UpdateSettingsRequest{
		DefaultQuota: &QuotaPolicy{
			Mode: "limited",
			GB:   15,
		},
		AuditRetentionDays: &retention,
		WorkerConcurrency:  &concurrency,
	}

	if err := handler.service.UpdateSystemSettings(ctx, updateReq); err != nil {
		t.Fatalf("failed to update settings: %v", err)
	}

	updated, err := handler.service.GetSystemSettings(ctx)
	if err != nil {
		t.Fatalf("failed to get updated settings: %v", err)
	}
	if updated.DefaultQuota.Mode != "limited" || updated.DefaultQuota.GB != 15 {
		t.Errorf("expected limited 15 GB, got %s %d", updated.DefaultQuota.Mode, updated.DefaultQuota.GB)
	}
	if updated.AuditRetentionDays != 30 {
		t.Errorf("expected audit retention 30, got %d", updated.AuditRetentionDays)
	}
	if updated.WorkerConcurrency != 4 {
		t.Errorf("expected worker concurrency 4, got %d", updated.WorkerConcurrency)
	}

	// 3. Validation errors
	// a. Limited quota < 1
	err = handler.service.UpdateSystemSettings(ctx, &UpdateSettingsRequest{
		DefaultQuota: &QuotaPolicy{
			Mode: "limited",
			GB:   0,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "at least 1 GB") {
		t.Errorf("expected error for quota < 1 GB, got %v", err)
	}

	// b. Invalid audit retention
	invalidRetention := 45
	err = handler.service.UpdateSystemSettings(ctx, &UpdateSettingsRequest{
		AuditRetentionDays: &invalidRetention,
	})
	if err == nil || !strings.Contains(err.Error(), "must be one of") {
		t.Errorf("expected error for invalid retention, got %v", err)
	}

	// c. Worker concurrency < 1 or > 10
	invalidConcurrency := 0
	err = handler.service.UpdateSystemSettings(ctx, &UpdateSettingsRequest{
		WorkerConcurrency: &invalidConcurrency,
	})
	if err == nil || !strings.Contains(err.Error(), "between 1 and 10") {
		t.Errorf("expected error for concurrency < 1, got %v", err)
	}

	invalidConcurrencyHigh := 11
	err = handler.service.UpdateSystemSettings(ctx, &UpdateSettingsRequest{
		WorkerConcurrency: &invalidConcurrencyHigh,
	})
	if err == nil || !strings.Contains(err.Error(), "between 1 and 10") {
		t.Errorf("expected error for concurrency > 10, got %v", err)
	}
}

func TestDownloadBackupHandler(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/backup", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.DownloadBackup(c)
	if err != nil {
		t.Fatalf("DownloadBackup failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	disposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment; filename=cubycloud.db") {
		t.Errorf("expected attachment disposition, got %s", disposition)
	}
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %s", contentType)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected non-empty database backup stream")
	}

	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(backupPath, rec.Body.Bytes(), 0o600); err != nil {
		t.Fatalf("failed to write backup stream: %v", err)
	}
	backupDB, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatalf("failed to open backup database: %v", err)
	}
	defer backupDB.Close()
	var integrity string
	if err := backupDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("failed to check backup integrity: %v", err)
	}
	if integrity != "ok" {
		t.Errorf("expected valid SQLite backup, integrity_check returned %q", integrity)
	}
	var users int
	if err := backupDB.QueryRow("SELECT count(*) FROM users").Scan(&users); err != nil {
		t.Fatalf("failed to query users from backup: %v", err)
	}
}

func TestUpdateSettingsHandler(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()

	e := echo.New()
	body := `{"default_quota":{"mode":"limited","gb":20},"audit_retention_days":60,"worker_concurrency":5}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdateSettings(c); err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp SystemSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.DefaultQuota.GB != 20 || resp.AuditRetentionDays != 60 || resp.WorkerConcurrency != 5 {
		t.Errorf("unexpected settings response: %+v", resp)
	}
}

type mockConcurrencyController struct {
	concurrency int
}

func (m *mockConcurrencyController) SetConcurrency(limit int) {
	m.concurrency = limit
}

func TestUpdateSettingsHandler_DynamicConcurrencyController(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()

	mockCtl := &mockConcurrencyController{concurrency: 2}
	handler.SetConcurrencyController(mockCtl)

	e := echo.New()
	body := `{"worker_concurrency":4}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdateSettings(c); err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if mockCtl.concurrency != 4 {
		t.Errorf("expected mock controller concurrency 4, got %d", mockCtl.concurrency)
	}
}

func TestStorageNodeNamesAndUsage(t *testing.T) {
	db, handler, dbPath := setupTestAdmin(t)
	ctx := context.Background()

	localPath := t.TempDir()
	local, err := handler.service.CreateLocalNode(ctx, &CreateLocalNodeRequest{
		Name: "  My Disk  ", Path: localPath, CapacityCapGB: 1, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if local.Name != "My Disk" {
		t.Fatalf("stored name = %q", local.Name)
	}
	if _, err := handler.service.CreateCloudNode(ctx, &CreateCloudNodeRequest{Name: "my disk"}); err == nil {
		t.Fatal("expected cross-type case-insensitive duplicate name to fail")
	}
	if _, err := handler.service.CreateLocalNode(ctx, &CreateLocalNodeRequest{Name: "  ", Path: localPath}); err == nil {
		t.Fatal("expected blank name to fail")
	}

	dataDir := filepath.Dir(filepath.Dir(dbPath))
	defaultPath := filepath.Join(dataDir, "storage", "default")
	nodeID := "usage-node"
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES(?,?,?,?,?,1)`, nodeID, "Default", "LOCAL", defaultPath, 2<<30); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users(id,username,email,password_hash,role) VALUES('usage-user','usage','usage@example.test','hash','REGULAR')`); err != nil {
		t.Fatal(err)
	}
	used := int64(2<<30) + 1
	for i, deletedAt := range []any{nil, "2026-01-01 00:00:00"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id,modified_at,deleted_at) VALUES(?,?,'file.bin','file',?,?,?,?,?)`,
			fmt.Sprintf("usage-item-%d", i), "usage-user", used, "same-physical-blob", nodeID, time.Now(), deletedAt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,capacity_bytes,enabled) VALUES('usage-s3','S3','S3',1048576,1)`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, key, node string
		size          int64
	}{
		{"org-local", "local-org-blob", nodeID, 100},
		{"org-local-shared", "same-physical-blob", nodeID, used},
		{"org-s3", "s3-blob", "usage-s3", 4096},
	} {
		if _, err := db.ExecContext(ctx, `INSERT INTO organization_items(id,created_by,name,kind,size_bytes,disk_key,storage_node_id) VALUES(?,'usage-user',?,'file',?,?,?)`,
			item.id, item.id, item.size, item.key, item.node); err != nil {
			t.Fatal(err)
		}
	}
	// Organization files count toward S3 node usage even when no My Storage reference exists.
	nodes, err := handler.service.GetStorageNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.ID == "usage-s3" && node.UsedBytes != 4096 {
			t.Fatalf("S3 used_bytes = %d, want organization-only usage 4096", node.UsedBytes)
		}
		if node.ID == nodeID && node.UsedBytes != used+100 {
			t.Fatalf("local used_bytes = %d, want My Storage plus organization usage with shared disk_key deduplicated (%d)", node.UsedBytes, used+100)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id) VALUES('s3-personal','usage-user','s3-personal.bin','file',4096,'s3-blob','usage-s3')`); err != nil {
		t.Fatal(err)
	}
	nodes, err = handler.service.GetStorageNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.ID == "usage-s3" && node.UsedBytes != 4096 {
			t.Fatalf("S3 used_bytes = %d, want shared My Storage/Organization disk_key counted once (4096)", node.UsedBytes)
		}
	}
	nodes, err = handler.service.GetStorageNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var reported int64 = -1
	for _, node := range nodes {
		if node.ID == nodeID {
			reported = node.UsedBytes
			break
		}
	}
	if reported != used+100 {
		t.Fatalf("used_bytes = %d, want My Storage plus organization usage with shared disk_key deduplicated (%d)", reported, used+100)
	}
	if err := handler.service.UpdateNode(ctx, nodeID, &UpdateNodeRequest{CapacityCapGB: int64Ptr(1)}); err == nil {
		t.Fatal("expected capacity below node usage to be rejected")
	}
}

func int64Ptr(v int64) *int64 { return &v }
