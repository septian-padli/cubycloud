package jobs

import (
	"context"
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
