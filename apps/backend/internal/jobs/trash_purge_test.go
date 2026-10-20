package jobs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/mystorage"
)

type mockStorageDriver struct {
	deletedKeys []string
	deleteErr   error
}

func (*mockStorageDriver) WriteLocal(_ context.Context, _, _ string, _ io.Reader) error {
	return nil
}

func (*mockStorageDriver) ReadLocal(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("content")), nil
}

func (m *mockStorageDriver) DeleteLocal(_ context.Context, _, key string) error {
	m.deletedKeys = append(m.deletedKeys, key)
	return nil
}

func (*mockStorageDriver) StatLocal(context.Context, string) (int64, int64, error) {
	return 1000000, 1000000, nil
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
func (m *mockStorageDriver) ReadObject(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("content")), nil
}

func (*mockStorageDriver) WriteObject(context.Context, string, string, io.Reader, int64) error {
	return nil
}

func (m *mockStorageDriver) DeleteObject(_ context.Context, _, key string) error {
	m.deletedKeys = append(m.deletedKeys, key)
	return m.deleteErr
}

func (*mockStorageDriver) HeadObject(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (*mockStorageDriver) PresignUpload(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (*mockStorageDriver) PresignDownload(context.Context, string, string, string, time.Duration) (string, error) {
	return "", nil
}

func TestTrashPurgeJobExpiredRetention(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"user-1", "testuser", "test@example.com", "hash", "REGULAR", 10000000)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dataDir, "storage", "default")
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('storage-node','Default','LOCAL',?,1073741824,1)`, nodePath); err != nil {
		t.Fatal(err)
	}

	driver := &mockStorageDriver{}
	service := mystorage.NewService(db, driver)
	ctx := context.Background()

	// Upload two files
	fOld, err := service.Upload(ctx, "user-1", "old-expired.txt", nil, strings.NewReader("old"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	fRecent, err := service.Upload(ctx, "user-1", "recent-trash.txt", nil, strings.NewReader("recent"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Delete both files
	if err := service.Delete(ctx, "user-1", fOld.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, "user-1", fRecent.ID); err != nil {
		t.Fatal(err)
	}

	// Update fOld deleted_at to 35 days ago
	oldTimestamp := time.Now().UTC().Add(-35 * 24 * time.Hour)
	_, err = db.Exec(`UPDATE my_storage_items SET deleted_at=? WHERE id=?`, oldTimestamp, fOld.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Update fRecent deleted_at to 5 days ago
	recentTimestamp := time.Now().UTC().Add(-5 * 24 * time.Hour)
	_, err = db.Exec(`UPDATE my_storage_items SET deleted_at=? WHERE id=?`, recentTimestamp, fRecent.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Start runner and trigger RunNow for 30 days retention
	runner := StartTrashPurgeJob(ctx, db, driver, 1*time.Hour, 30*24*time.Hour)
	defer runner.Stop()

	purged, err := runner.RunNow(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error during purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("expected 1 item purged, got %d", purged)
	}

	// Verify old item was physically purged
	foundOldKey := false
	for _, k := range driver.deletedKeys {
		if k == fOld.ID {
			foundOldKey = true
		}
	}
	if !foundOldKey {
		t.Fatalf("expected old file disk key to be deleted from physical storage")
	}

	// Verify recent item is still in trash
	trashItems, err := service.ListTrash(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(trashItems) != 1 || trashItems[0].ID != fRecent.ID {
		t.Fatalf("expected recent item to remain in trash, got %d items", len(trashItems))
	}
}

func TestCleanupExpiredPendingCloudUploads(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO storage_nodes(id,name,type,capacity_bytes,enabled) VALUES('upload-node','Node','S3',1000,1)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id         string
		expiration string
	}{
		{"expired", "datetime('now','-1 second')"},
		{"unexpired", "datetime('now','+1 hour')"},
	} {
		if _, err := db.Exec(`INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES(?, 'owner', 'upload-node', 'file', 1, `+row.expiration+`)`, row.id); err != nil {
			t.Fatal(err)
		}
	}
	driver := &mockStorageDriver{deleteErr: errors.New("S3 unavailable")}
	runner := &TrashPurgeRunner{service: mystorage.NewService(db, driver)}
	runner.cleanupExpiredPendingCloudUploads(context.Background())

	var expired, unexpired int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='expired'`).Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if expired != 1 {
		t.Fatalf("failed object delete removed expired intent; remaining = %d, want 1", expired)
	}

	driver.deleteErr = nil
	runner.cleanupExpiredPendingCloudUploads(context.Background())
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='expired'`).Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='unexpired'`).Scan(&unexpired); err != nil {
		t.Fatal(err)
	}
	if expired != 0 || unexpired != 1 {
		t.Fatalf("cleanup left expired=%d and unexpired=%d, want 0 and 1", expired, unexpired)
	}
	if len(driver.deletedKeys) != 2 || driver.deletedKeys[0] != "expired" || driver.deletedKeys[1] != "expired" {
		t.Fatalf("deleted objects = %v, want two retry attempts for expired", driver.deletedKeys)
	}
}

func TestTrashPurgeJobExpiredOrganizationAndMyStorage(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"user-1", "testuser", "test@example.com", "hash", "REGULAR", 10000000)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dataDir, "storage", "default")
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('storage-node','Default','LOCAL',?,1073741824,1)`, nodePath); err != nil {
		t.Fatal(err)
	}

	driver := &mockStorageDriver{}
	mySvc := mystorage.NewService(db, driver)
	ctx := context.Background()

	// 1. Create file in my_storage
	fMy, err := mySvc.Upload(ctx, "user-1", "my-expired.txt", nil, strings.NewReader("my content"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := mySvc.Delete(ctx, "user-1", fMy.ID); err != nil {
		t.Fatal(err)
	}

	// 2. Create file in organization_items directly
	orgFileID := "org-expired-id"
	orgDiskKey := "org-expired-disk-key"
	oldTimestamp := time.Now().UTC().Add(-35 * 24 * time.Hour)
	_, err = db.Exec(`INSERT INTO organization_items(id, created_by, deleted_by, name, kind, size_bytes, disk_key, storage_node_id, deleted_at) VALUES(?, ?, ?, ?, 'file', 100, ?, 'storage-node', ?)`,
		orgFileID, "user-1", "user-1", "org-expired.txt", orgDiskKey, oldTimestamp)
	if err != nil {
		t.Fatal(err)
	}

	// Update fMy deleted_at to 35 days ago as well
	_, err = db.Exec(`UPDATE my_storage_items SET deleted_at=? WHERE id=?`, oldTimestamp, fMy.ID)
	if err != nil {
		t.Fatal(err)
	}

	runner := StartTrashPurgeJob(ctx, db, driver, 1*time.Hour, 30*24*time.Hour)
	defer runner.Stop()

	purged, err := runner.RunNow(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error during purge: %v", err)
	}
	if purged != 2 {
		t.Fatalf("expected 2 items purged (1 my_storage + 1 org), got %d", purged)
	}

	// Verify both disk keys were physically deleted
	foundMyKey := false
	foundOrgKey := false
	for _, k := range driver.deletedKeys {
		if k == fMy.ID {
			foundMyKey = true
		}
		if k == orgDiskKey {
			foundOrgKey = true
		}
	}
	if !foundMyKey {
		t.Fatalf("expected my_storage file disk key to be deleted physically")
	}
	if !foundOrgKey {
		t.Fatalf("expected organization file disk key to be deleted physically")
	}

	// Verify both items are completely gone from DB
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE id=?`, fMy.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected my_storage item to be removed from DB")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM organization_items WHERE id=?`, orgFileID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected organization item to be removed from DB")
	}
}
