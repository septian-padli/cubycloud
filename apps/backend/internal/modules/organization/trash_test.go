package organization_test

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
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/uptrace/bun"
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

func setupTestDB(t *testing.T) (*bun.DB, *mystorage.Service, *organization.Service, *mockStorageDriver) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"user-1", "testuser", "test@example.com", "hash", "REGULAR", 10000000)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"user-2", "user2", "user2@example.com", "hash", "REGULAR", 10000000)
	if err != nil {
		t.Fatal(err)
	}

	nodePath := filepath.Join(dataDir, "storage", "default")
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('storage-node','Default','LOCAL',?,1073741824,1)`, nodePath); err != nil {
		t.Fatal(err)
	}

	driver := &mockStorageDriver{}
	myStorageService := mystorage.NewService(db, driver)
	orgService := organization.NewService(db, driver)
	return db, myStorageService, orgService, driver
}

func TestOrganizationTrashLifecycle(t *testing.T) {
	db, myStorageService, orgService, driver := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create org folder and child file
	folder, _, err := orgService.CreateFolder(ctx, "user-1", "org-folder", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	file, err := orgService.Upload(ctx, "user-1", "org-file.txt", &folder.ID, strings.NewReader("org content"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 2. user-1 deletes the child file
	if err := orgService.Delete(ctx, "user-1", file.ID); err != nil {
		t.Fatal(err)
	}

	// 3. Verify it appears in user-1's trash, but NOT in user-2's trash
	u1Trash, err := myStorageService.ListTrash(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(u1Trash) != 1 || u1Trash[0].ID != file.ID {
		t.Fatalf("expected org-file in user-1 trash, got %d items", len(u1Trash))
	}
	// Since org-folder is still active, original_parent_missing should be false
	if u1Trash[0].OriginalParentMissing {
		t.Fatalf("expected original_parent_missing to be false while folder is active")
	}

	u2Trash, err := myStorageService.ListTrash(ctx, "user-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(u2Trash) != 0 {
		t.Fatalf("expected user-2 trash to be empty, got %d items", len(u2Trash))
	}

	// 4. Delete the parent folder as well
	if err := orgService.Delete(ctx, "user-1", folder.ID); err != nil {
		t.Fatal(err)
	}

	// Now file's original_parent_missing should be true because parent is in trash
	u1Trash, err = myStorageService.ListTrash(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(u1Trash) != 2 {
		t.Fatalf("expected 2 items in user-1 trash, got %d", len(u1Trash))
	}
	for _, it := range u1Trash {
		if it.ID == file.ID && !it.OriginalParentMissing {
			t.Fatalf("expected original_parent_missing=true for file when folder is deleted")
		}
	}

	// 5. Restore file with parent deleted -> restored to root
	restoredFile, restoredToRoot, err := myStorageService.Restore(ctx, "user-1", file.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !restoredToRoot {
		t.Fatalf("expected restoredToRoot=true when parent is deleted")
	}
	if restoredFile.ParentID != nil {
		t.Fatalf("expected restored file to be at root (parent_id nil), got %v", restoredFile.ParentID)
	}

	// Verify file is back in organization active items
	orgItems, _, err := orgService.List(ctx, "user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range orgItems {
		if it.ID == file.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("restored file not found in organization root items")
	}

	// 6. Test PurgeItem with shared disk_key between my_storage and organization
	// Re-delete the file to trash
	if err := orgService.Delete(ctx, "user-1", file.ID); err != nil {
		t.Fatal(err)
	}

	// Create a duplicate/shared disk_key in my_storage_items
	var diskKey string
	err = db.QueryRow(`SELECT disk_key FROM organization_items WHERE id=?`, file.ID).Scan(&diskKey)
	if err != nil {
		t.Fatal(err)
	}
	myFile, err := myStorageService.Upload(ctx, "user-1", "shared-file.txt", nil, strings.NewReader("shared content"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Point myFile's disk_key to diskKey
	_, err = db.Exec(`UPDATE my_storage_items SET disk_key=? WHERE id=?`, diskKey, myFile.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Purge the org item: disk_key must NOT be physically deleted because myFile uses it
	if err := myStorageService.PurgeItem(ctx, "user-1", file.ID); err != nil {
		t.Fatal(err)
	}
	for _, k := range driver.deletedKeys {
		if k == diskKey {
			t.Fatalf("disk key was physically deleted while still referenced in my_storage_items")
		}
	}

	// Now delete and purge myFile
	if err := myStorageService.Delete(ctx, "user-1", myFile.ID); err != nil {
		t.Fatal(err)
	}
	if err := myStorageService.PurgeItem(ctx, "user-1", myFile.ID); err != nil {
		t.Fatal(err)
	}
	// Now diskKey should be physically deleted
	foundKey := false
	for _, k := range driver.deletedKeys {
		if k == diskKey {
			foundKey = true
			break
		}
	}
	if !foundKey {
		t.Fatalf("expected diskKey to be physically deleted after all references removed")
	}
}
