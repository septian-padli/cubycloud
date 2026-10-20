package mystorage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
)

func TestHome_QuickAccessOrderingAndLimit(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	user1 := "user-1"
	user2 := "user-2"
	_, err := db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		user2, "testuser2", "test2@example.com", "hash", "REGULAR", 10000000)
	if err != nil {
		t.Fatal(err)
	}
	var fileIDs []string
	for i := 1; i <= 6; i++ {
		name := strings.Repeat("a", i) + ".txt"
		item, err := service.Upload(ctx, user1, name, nil, strings.NewReader("content"), false, 1000, nil)
		if err != nil {
			t.Fatalf("failed to upload %s: %v", name, err)
		}
		fileIDs = append(fileIDs, item.ID)
	}

	// Record access with simulated timestamps
	baseTime := time.Now().UTC().Add(-1 * time.Hour)
	for i, id := range fileIDs {
		// Record access directly with increasing timestamps
		accTime := baseTime.Add(time.Duration(i*10) * time.Minute)
		_, err := db.DB.ExecContext(ctx, `
			INSERT INTO vfs_recent_access (id, user_id, item_id, source, accessed_at)
			VALUES (?, ?, ?, 'my-storage', ?)
		`, "acc-"+id, user1, id, accTime)
		if err != nil {
			t.Fatalf("failed to record access for %s: %v", id, err)
		}
	}

	// Also record access for user2 on an item
	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO vfs_recent_access (id, user_id, item_id, source, accessed_at)
		VALUES ('acc-u2', ?, ?, 'my-storage', ?)
	`, user2, fileIDs[0], time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to record access for user2: %v", err)
	}

	// Fetch home for user1
	home, err := service.GetHome(ctx, user1)
	if err != nil {
		t.Fatalf("GetHome failed: %v", err)
	}

	// Must cap at 5 items
	if len(home.QuickAccess) != 5 {
		t.Fatalf("expected 5 quick access items, got %d", len(home.QuickAccess))
	}

	// Order must be latest accessed first (fileIDs[5], fileIDs[4], fileIDs[3], fileIDs[2], fileIDs[1])
	expectedOrder := []string{fileIDs[5], fileIDs[4], fileIDs[3], fileIDs[2], fileIDs[1]}
	for i, expectedID := range expectedOrder {
		if home.QuickAccess[i].ID != expectedID {
			t.Errorf("at index %d: expected item ID %s, got %s", i, expectedID, home.QuickAccess[i].ID)
		}
	}

	// Trash the most recent file and verify it disappears from quick access
	err = service.Delete(ctx, user1, fileIDs[5])
	if err != nil {
		t.Fatalf("failed to trash file: %v", err)
	}

	homeAfterTrash, err := service.GetHome(ctx, user1)
	if err != nil {
		t.Fatalf("GetHome after trash failed: %v", err)
	}
	for _, it := range homeAfterTrash.QuickAccess {
		if it.ID == fileIDs[5] {
			t.Errorf("trashed file %s still appeared in quick access", fileIDs[5])
		}
	}
}

func TestHome_ActivitiesDeduplicationAndOrgFilter(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	user1 := "user-1"
	user2 := "user-2"
	_, err := db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		user2, "testuser2", "test2@example.com", "hash", "REGULAR", 10000000)
	if err != nil {
		t.Fatal(err)
	}
	doc1, err := service.Upload(ctx, user1, "doc1.txt", nil, strings.NewReader("content"), false, 1000, nil)
	if err != nil {
		t.Fatalf("failed to upload doc1: %v", err)
	}

	// 2. User1 renames doc1.txt -> Action: edit
	newName := "doc1-renamed.txt"
	_, _, err = service.Update(ctx, user1, doc1.ID, &newName, nil, false)
	if err != nil {
		t.Fatalf("failed to rename doc1: %v", err)
	}

	// 3. User1 creates a folder -> Action: upload (folder)
	folder1, _, err := service.CreateFolder(ctx, user1, "Folder A", nil, false)
	if err != nil {
		t.Fatalf("failed to create folder: %v", err)
	}

	// 4. User1 moves doc1 into folder1 -> Action: edit
	_, _, err = service.MoveItem(ctx, user1, doc1.ID, &folder1.ID)
	if err != nil {
		t.Fatalf("failed to move doc1: %v", err)
	}

	// 5. In organization: User1 uploads an item
	orgItem1ID := "org-item-user1"
	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, modified_at)
		VALUES (?, ?, 'shared-report.pdf', 'file', 2048, NULL, ?)
	`, orgItem1ID, user1, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to insert org item for user1: %v", err)
	}
	_ = database.RecordActivity(ctx, db.DB, user1, "upload", orgItem1ID, "shared-report.pdf", "file", "organization")

	// 6. In organization: User2 uploads an item
	orgItem2ID := "org-item-user2"
	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, modified_at)
		VALUES (?, ?, 'other-secret.docx', 'file', 4096, NULL, ?)
	`, orgItem2ID, user2, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to insert org item for user2: %v", err)
	}
	_ = database.RecordActivity(ctx, db.DB, user2, "upload", orgItem2ID, "other-secret.docx", "file", "organization")

	// Check Home for user1
	home, err := service.GetHome(ctx, user1)
	if err != nil {
		t.Fatalf("GetHome failed: %v", err)
	}

	// Verify User2's organization activity does NOT appear for User1
	for _, act := range home.Activities {
		if act.ItemID == orgItem2ID || act.FileName == "other-secret.docx" {
			t.Errorf("User2's organization item appeared in User1's activities: %+v", act)
		}
	}

	// Verify deduplication: doc1 was uploaded, renamed, and moved.
	// It must only appear ONCE in activities, with action='edit' and the latest name 'doc1-renamed.txt'
	var doc1Appearances int
	var doc1Action string
	var doc1FileName string
	for _, act := range home.Activities {
		if act.ItemID == doc1.ID {
			doc1Appearances++
			doc1Action = act.Action
			doc1FileName = act.FileName
		}
	}

	if doc1Appearances != 1 {
		t.Fatalf("expected doc1 to appear exactly once, got %d times", doc1Appearances)
	}
	if doc1Action != "edit" {
		t.Errorf("expected doc1 action to be 'edit', got '%s'", doc1Action)
	}
	if doc1FileName != "doc1-renamed.txt" {
		t.Errorf("expected doc1 filename to be 'doc1-renamed.txt', got '%s'", doc1FileName)
	}
}
