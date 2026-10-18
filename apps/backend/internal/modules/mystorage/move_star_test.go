package mystorage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestMoveItemSuccess(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	// Setup folders and file
	f1, _, err := service.CreateFolder(ctx, owner, "Folder 1", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	f2, _, err := service.CreateFolder(ctx, owner, "Folder 2", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := service.Upload(ctx, owner, "doc.txt", nil, strings.NewReader("hello doc"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Move doc.txt into f1
	moved, _, err := service.MoveItem(ctx, owner, doc.ID, &f1.ID)
	if err != nil {
		t.Fatalf("failed to move to f1: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != f1.ID {
		t.Fatalf("expected parent %s, got %v", f1.ID, moved.ParentID)
	}

	// Check root: doc.txt should no longer be in root
	rootItems, _, err := service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range rootItems {
		if it.ID == doc.ID {
			t.Fatalf("doc.txt still visible in root")
		}
	}

	// Check f1: doc.txt must be inside f1
	f1Items, _, err := service.List(ctx, owner, &f1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(f1Items) != 1 || f1Items[0].ID != doc.ID {
		t.Fatalf("expected doc.txt in f1, got %v", f1Items)
	}

	// 2. Move doc.txt from f1 to f2
	moved, _, err = service.MoveItem(ctx, owner, doc.ID, &f2.ID)
	if err != nil {
		t.Fatalf("failed to move to f2: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != f2.ID {
		t.Fatalf("expected parent %s, got %v", f2.ID, moved.ParentID)
	}

	f2Items, _, err := service.List(ctx, owner, &f2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2Items) != 1 || f2Items[0].ID != doc.ID {
		t.Fatalf("expected doc.txt in f2, got %v", f2Items)
	}

	// 3. Move doc.txt back to root (targetParentID = nil)
	moved, _, err = service.MoveItem(ctx, owner, doc.ID, nil)
	if err != nil {
		t.Fatalf("failed to move to root: %v", err)
	}
	if moved.ParentID != nil {
		t.Fatalf("expected parent nil, got %v", moved.ParentID)
	}

	rootItems, _, err = service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	foundDoc := false
	for _, it := range rootItems {
		if it.ID == doc.ID {
			foundDoc = true
			break
		}
	}
	if !foundDoc {
		t.Fatalf("expected doc.txt back in root")
	}
}

func TestMoveItemCircularMove(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	parent, _, err := service.CreateFolder(ctx, owner, "parent", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := service.CreateFolder(ctx, owner, "child", &parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	subchild, _, err := service.CreateFolder(ctx, owner, "subchild", &child.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	// Move folder into itself
	_, _, err = service.MoveItem(ctx, owner, parent.ID, &parent.ID)
	if !errors.Is(err, ErrCircularMove) {
		t.Fatalf("expected ErrCircularMove for moving into itself, got %v", err)
	}

	// Move folder into its child
	_, _, err = service.MoveItem(ctx, owner, parent.ID, &child.ID)
	if !errors.Is(err, ErrCircularMove) {
		t.Fatalf("expected ErrCircularMove for moving into child, got %v", err)
	}

	// Move folder into its subchild
	_, _, err = service.MoveItem(ctx, owner, parent.ID, &subchild.ID)
	if !errors.Is(err, ErrCircularMove) {
		t.Fatalf("expected ErrCircularMove for moving into subchild, got %v", err)
	}

	// Move child into subchild
	_, _, err = service.MoveItem(ctx, owner, child.ID, &subchild.ID)
	if !errors.Is(err, ErrCircularMove) {
		t.Fatalf("expected ErrCircularMove for moving child into subchild, got %v", err)
	}

	// Moving child to root should succeed
	moved, _, err := service.MoveItem(ctx, owner, child.ID, nil)
	if err != nil {
		t.Fatalf("expected moving child to root to succeed, got %v", err)
	}
	if moved.ParentID != nil {
		t.Fatalf("expected child parent nil, got %v", moved.ParentID)
	}
}

func TestMoveItemNameConflict(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	target, _, err := service.CreateFolder(ctx, owner, "target", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Existing file in target
	_, err = service.Upload(ctx, owner, "report.pdf", &target.ID, strings.NewReader("existing in target"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// File in root with same name
	fileToMove, err := service.Upload(ctx, owner, "report.pdf", nil, strings.NewReader("file in root"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Try moving fileToMove into target -> must conflict
	_, suggestion, err := service.MoveItem(ctx, owner, fileToMove.ID, &target.ID)
	if !errors.Is(err, ErrNameConflict) {
		t.Fatalf("expected ErrNameConflict, got %v", err)
	}
	if suggestion == "" {
		t.Fatalf("expected non-empty suggestion for conflict, got empty")
	}
	// Folder conflict test
	_, _, err = service.CreateFolder(ctx, owner, "docs", &target.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	folderToMove, _, err := service.CreateFolder(ctx, owner, "docs", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = service.MoveItem(ctx, owner, folderToMove.ID, &target.ID)
	if !errors.Is(err, ErrNameConflict) {
		t.Fatalf("expected ErrNameConflict for folder, got %v", err)
	}
}

func TestMoveItemInvalidTarget(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	file1, err := service.Upload(ctx, owner, "file1.txt", nil, strings.NewReader("data"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	file2, err := service.Upload(ctx, owner, "file2.txt", nil, strings.NewReader("data"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Moving to a file (not folder)
	_, _, err = service.MoveItem(ctx, owner, file1.ID, &file2.ID)
	if !errors.Is(err, ErrBadParent) {
		t.Fatalf("expected ErrBadParent when target is file, got %v", err)
	}

	// Moving to non-existent folder
	nonExistent := "non-existent-id"
	_, _, err = service.MoveItem(ctx, owner, file1.ID, &nonExistent)
	if !errors.Is(err, ErrBadParent) {
		t.Fatalf("expected ErrBadParent when target does not exist, got %v", err)
	}

	// Moving to a deleted folder (in trash)
	trashFolder, _, err := service.CreateFolder(ctx, owner, "trash-folder", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Delete(ctx, owner, trashFolder.ID); err != nil {
		t.Fatal(err)
	}
	_, _, err = service.MoveItem(ctx, owner, file1.ID, &trashFolder.ID)
	if !errors.Is(err, ErrBadParent) {
		t.Fatalf("expected ErrBadParent when target is trashed, got %v", err)
	}

	// Moving a non-existent item
	_, _, err = service.MoveItem(ctx, owner, "does-not-exist", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent item, got %v", err)
	}
}

func TestToggleStar(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	file, err := service.Upload(ctx, owner, "notes.txt", nil, strings.NewReader("notes content"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Initial: not starred
	items, _, err := service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].IsStarred {
		t.Fatalf("expected IsStarred to be false initially")
	}

	// 2. Star item
	isStarred, err := service.ToggleStar(ctx, owner, file.ID)
	if err != nil {
		t.Fatalf("ToggleStar failed: %v", err)
	}
	if !isStarred {
		t.Fatalf("expected isStarred to be true after star")
	}

	// Verify List reflection
	items, _, err = service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || !items[0].IsStarred {
		t.Fatalf("expected IsStarred to be true in List")
	}

	// 3. Unstar item
	isStarred, err = service.ToggleStar(ctx, owner, file.ID)
	if err != nil {
		t.Fatalf("ToggleStar unstar failed: %v", err)
	}
	if isStarred {
		t.Fatalf("expected isStarred to be false after unstar")
	}

	items, _, err = service.List(ctx, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].IsStarred {
		t.Fatalf("expected IsStarred to be false in List after unstar")
	}

	// 4. Star again
	isStarred, err = service.ToggleStar(ctx, owner, file.ID)
	if err != nil {
		t.Fatalf("ToggleStar re-star failed: %v", err)
	}
	if !isStarred {
		t.Fatalf("expected isStarred to be true after re-star")
	}

	// 5. Toggle non-existent item -> ErrNotFound
	_, err = service.ToggleStar(ctx, owner, "non-existent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent item, got %v", err)
	}
}

func TestListStarred(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	owner := "user-1"

	item1, err := service.Upload(ctx, owner, "item1.txt", nil, strings.NewReader("data 1"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	item2, err := service.Upload(ctx, owner, "item2.txt", nil, strings.NewReader("data 2"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	item3, _, err := service.CreateFolder(ctx, owner, "folder3", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Star item1
	_, err = service.ToggleStar(ctx, owner, item1.ID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // ensure different starred_at timestamp
	// Star item2
	_, err = service.ToggleStar(ctx, owner, item2.ID)
	if err != nil {
		t.Fatal(err)
	}

	// item3 is not starred
	_ = item3

	starred, err := service.ListStarred(ctx, owner)
	if err != nil {
		t.Fatalf("ListStarred failed: %v", err)
	}
	if len(starred) != 2 {
		t.Fatalf("expected 2 starred items, got %d", len(starred))
	}
	// Ordered by starred_at DESC: item2 was starred after item1
	if starred[0].ID != item2.ID || starred[1].ID != item1.ID {
		t.Fatalf("expected order [item2, item1], got [%s, %s]", starred[0].ID, starred[1].ID)
	}
	if !starred[0].IsStarred || !starred[1].IsStarred {
		t.Fatalf("expected IsStarred to be true for all returned items")
	}

	// Soft-delete item2 (move to trash)
	if err = service.Delete(ctx, owner, item2.ID); err != nil {
		t.Fatal(err)
	}

	// ListStarred must ignore trashed items!
	starredAfterDelete, err := service.ListStarred(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(starredAfterDelete) != 1 || starredAfterDelete[0].ID != item1.ID {
		t.Fatalf("expected only item1 in starred after item2 trashed, got %v", starredAfterDelete)
	}

	// Purge item2 completely: verify vfs_starred is cleaned up
	if err = service.PurgeItem(ctx, owner, item2.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM vfs_starred WHERE node_id = ?", item2.ID).Scan(&count)
	if count != 0 {
		t.Fatalf("expected vfs_starred row to be purged, got count %d", count)
	}

	// Unstar item1
	_, err = service.ToggleStar(ctx, owner, item1.ID)
	if err != nil {
		t.Fatal(err)
	}
	starredEmpty, err := service.ListStarred(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(starredEmpty) != 0 {
		t.Fatalf("expected 0 starred items, got %d", len(starredEmpty))
	}
}

func TestHTTPMoveStarEndpoints(t *testing.T) {
	db, service, _ := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	ownerID := "user-1"

	handler := NewHandler(service)
	e := echo.New()

	folder1, _, err := service.CreateFolder(ctx, ownerID, "TargetFolder", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	file1, err := service.Upload(ctx, ownerID, "testfile.txt", nil, strings.NewReader("content"), false, 1000, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 1. HTTP MoveItem success
	{
		bodyJSON := `{"target_parent_id":"` + folder1.ID + `"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/items/"+file1.ID+"/move", strings.NewReader(bodyJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", ownerID)
		c.SetParamNames("id")
		c.SetParamValues(file1.ID)

		if err := handler.MoveItem(c); err != nil {
			t.Fatalf("MoveItem HTTP handler error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var node VFSNode
		if err := json.Unmarshal(rec.Body.Bytes(), &node); err != nil {
			t.Fatal(err)
		}
		if node.ParentID == nil || *node.ParentID != folder1.ID {
			t.Fatalf("expected node parent %s, got %v", folder1.ID, node.ParentID)
		}
	}

	// 2. HTTP MoveItem Conflict 409
	{
		// Create collision in root
		file2, err := service.Upload(ctx, ownerID, "collision.txt", nil, strings.NewReader("root content"), false, 1000, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.Upload(ctx, ownerID, "collision.txt", &folder1.ID, strings.NewReader("folder content"), false, 1000, nil)
		if err != nil {
			t.Fatal(err)
		}

		bodyJSON := `{"target_parent_id":"` + folder1.ID + `"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/items/"+file2.ID+"/move", strings.NewReader(bodyJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", ownerID)
		c.SetParamNames("id")
		c.SetParamValues(file2.ID)

		if err := handler.MoveItem(c); err != nil {
			t.Fatalf("MoveItem HTTP handler unexpected error: %v", err)
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d: %s", rec.Code, rec.Body.String())
		}
		var errResp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
		if errResp["error"] != "name already in use" {
			t.Fatalf("expected error 'name already in use', got %v", errResp)
		}
	}

	// 3. HTTP MoveItem Circular 400
	{
		bodyJSON := `{"target_parent_id":"` + folder1.ID + `"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/items/"+folder1.ID+"/move", strings.NewReader(bodyJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", ownerID)
		c.SetParamNames("id")
		c.SetParamValues(folder1.ID)

		if err := handler.MoveItem(c); err != nil {
			t.Fatalf("MoveItem HTTP handler unexpected error: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 4. HTTP ToggleStar
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/items/"+file1.ID+"/star", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", ownerID)
		c.SetParamNames("id")
		c.SetParamValues(file1.ID)

		if err := handler.ToggleStar(c); err != nil {
			t.Fatalf("ToggleStar HTTP handler error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var starResp map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &starResp)
		if starResp["is_starred"] != true || starResp["id"] != file1.ID {
			t.Fatalf("unexpected star response: %v", starResp)
		}
	}

	// 5. HTTP ListStarred
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/starred", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", ownerID)

		if err := handler.ListStarred(c); err != nil {
			t.Fatalf("ListStarred HTTP handler error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var listResp struct {
			Items []VFSNode `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
			t.Fatal(err)
		}
		if len(listResp.Items) != 1 || listResp.Items[0].ID != file1.ID {
			t.Fatalf("expected file1 in starred items, got %v", listResp.Items)
		}
	}
}
