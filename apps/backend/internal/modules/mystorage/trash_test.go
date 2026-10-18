package mystorage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
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

func setupTestDB(t *testing.T) (*bun.DB, *Service, *mockStorageDriver) {
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
	nodePath := filepath.Join(dataDir, "storage", "default")
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('storage-node','Default','LOCAL',?,1073741824,1)`, nodePath); err != nil {
		t.Fatal(err)
	}

	driver := &mockStorageDriver{}
	service := NewService(db, driver)
	return db, service, driver
}

func TestTrashLifecycleAndOrdering(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	// 1. Create items
	folder, _, err := service.CreateFolder(ctx, owner, "projects", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	f1, err := service.Upload(ctx, owner, "root-file.txt", nil, strings.NewReader("hello root"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	f2, err := service.Upload(ctx, owner, "nested-file.txt", &folder.ID, strings.NewReader("hello nested"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Soft delete root-file.txt first
	if err := service.Delete(ctx, owner, f1.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	// Soft delete projects folder (containing nested-file.txt) second
	if err := service.Delete(ctx, owner, folder.ID); err != nil {
		t.Fatal(err)
	}

	// 3. List trash: projects, nested-file, and root-file.txt are in trash.
	// projects and nested-file were deleted together second, root-file.txt was deleted first.
	// Ordering is deleted_at DESC.
	trashItems, err := service.ListTrash(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(trashItems) != 3 {
		t.Fatalf("expected 3 trash items, got %d", len(trashItems))
	}
	// Last item in list must be root-file.txt (oldest deleted)
	if trashItems[len(trashItems)-1].ID != f1.ID {
		t.Fatalf("expected oldest item to be root-file.txt, got %s", trashItems[len(trashItems)-1].Name)
	}

	// 4. Restore projects folder
	restoredFolder, restoredToRoot, err := service.Restore(ctx, owner, folder.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if restoredToRoot {
		t.Fatalf("expected restoredToRoot to be false for root folder")
	}
	if restoredFolder.ID != folder.ID {
		t.Fatalf("restored ID mismatch")
	}

	// Verify projects and nested-file are back in My Storage
	activeItems, _, err := service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundFolder := false
	for _, it := range activeItems {
		if it.ID == folder.ID {
			foundFolder = true
		}
	}
	if !foundFolder {
		t.Fatalf("projects folder not found in active items after restore")
	}

	// Verify nested-file is back inside projects
	nestedItems, _, err := service.List(ctx, owner, &folder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedItems) != 1 || nestedItems[0].ID != f2.ID {
		t.Fatalf("nested-file not found in projects after restore")
	}

	// Trash should now only have root-file.txt
	trashItems, err = service.ListTrash(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(trashItems) != 1 || trashItems[0].ID != f1.ID {
		t.Fatalf("expected 1 item in trash, got %d", len(trashItems))
	}
}

func TestTrashRestoreMissingParentFallback(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	folder, _, err := service.CreateFolder(ctx, owner, "parent-folder", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	f, err := service.Upload(ctx, owner, "doc.txt", &folder.ID, strings.NewReader("content"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Soft delete the file inside parent-folder
	if err := service.Delete(ctx, owner, f.ID); err != nil {
		t.Fatal(err)
	}

	// Verify original_parent_missing is false while parent-folder is active
	trashItems, err := service.ListTrash(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(trashItems) != 1 || trashItems[0].OriginalParentMissing {
		t.Fatalf("expected original_parent_missing=false while parent is active")
	}

	// Soft delete parent-folder to trash
	if err := service.Delete(ctx, owner, folder.ID); err != nil {
		t.Fatal(err)
	}

	// List trash: parent is in trash, so doc.txt must have original_parent_missing=true
	trashItems, err = service.ListTrash(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	var docTrashItem *TrashItem
	for i := range trashItems {
		if trashItems[i].ID == f.ID {
			docTrashItem = &trashItems[i]
			break
		}
	}
	if docTrashItem == nil || !docTrashItem.OriginalParentMissing {
		t.Fatalf("expected original_parent_missing=true after parent is in trash")
	}

	// Restore doc.txt: since original parent is gone, it must fallback to root
	restored, restoredToRoot, err := service.Restore(ctx, owner, f.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !restoredToRoot {
		t.Fatalf("expected restoredToRoot=true when parent is missing")
	}
	if restored.ParentID != nil {
		t.Fatalf("expected restored item parent_id=nil (root), got %v", restored.ParentID)
	}

	// Verify doc.txt is now at root of My Storage
	rootItems, _, err := service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range rootItems {
		if it.ID == f.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("doc.txt not found in root after fallback restore")
	}
}

func TestTrashPurgeItemAndPurgeAll(t *testing.T) {
	db, service, driver := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	f1, err := service.Upload(ctx, owner, "f1.txt", nil, strings.NewReader("one"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Duplicate f1 (shares disk key)
	f1Dup, err := service.Duplicate(ctx, owner, f1.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = f1Dup

	f2, err := service.Upload(ctx, owner, "f2.txt", nil, strings.NewReader("two"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Delete both f1 and f2 to trash
	if err := service.Delete(ctx, owner, f1.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, owner, f2.ID); err != nil {
		t.Fatal(err)
	}

	// Purge f1 from trash: disk_key should NOT be deleted physically because f1Dup still references it!
	if err := service.PurgeItem(ctx, owner, f1.ID); err != nil {
		t.Fatal(err)
	}
	for _, k := range driver.deletedKeys {
		if k == f1.ID {
			t.Fatalf("disk key was prematurely deleted when duplicate still exists")
		}
	}

	// Now purge all trash (which contains f2)
	if err := service.PurgeAll(ctx, owner); err != nil {
		t.Fatal(err)
	}

	// f2 disk key should have been deleted physically
	foundF2Key := false
	for _, k := range driver.deletedKeys {
		if k == f2.ID {
			foundF2Key = true
		}
	}
	if !foundF2Key {
		t.Fatalf("expected f2 disk key to be deleted physically from storage")
	}

	// Trash should now be empty
	items, err := service.ListTrash(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items in trash after PurgeAll, got %d", len(items))
	}
}
