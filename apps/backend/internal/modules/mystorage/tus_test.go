package mystorage

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cubycloud/backend/internal/database"
	"github.com/labstack/echo/v4"
)

func setupTusTestEnvironment(t *testing.T, quotaBytes int64) (*Handler, *echo.Echo, string) {
	dataDir := t.TempDir()
	tusTemp := filepath.Join(dataDir, "tus_temp")
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tusTemp, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEMP_TUS_PATH", tusTemp)

	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"test-user", "testuser", "test@example.com", "hash", "REGULAR", quotaBytes)
	if err != nil {
		t.Fatal(err)
	}

	storageDir := filepath.Join(dataDir, "storage_default")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO storage_nodes(id, name, type, path, capacity_bytes, enabled) VALUES(?, ?, 'LOCAL', ?, ?, 1)`,
		"node-1", "Local Storage", storageDir, 1000000)
	if err != nil {
		t.Fatal(err)
	}

	driver := &uploadLimitDriver{capacity: 1000000, free: 900000}
	service := NewService(db, driver)
	handler := NewHandler(service)

	e := echo.New()
	g := e.Group("/api/v1/drive/my-storage")
	// Mock auth by injecting user_id to context
	g.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", "test-user")
			return next(c)
		}
	})
	handler.RegisterTusRoutes(g)

	return handler, e, tusTemp
}

func TestTusOptions(t *testing.T) {
	_, e, _ := setupTusTestEnvironment(t, 10000)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/drive/my-storage/upload-tus", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", rec.Code)
	}
	if rec.Header().Get("Tus-Resumable") != "1.0.0" {
		t.Errorf("expected Tus-Resumable: 1.0.0, got %s", rec.Header().Get("Tus-Resumable"))
	}
	if rec.Header().Get("Tus-Version") != "1.0.0" {
		t.Errorf("expected Tus-Version: 1.0.0, got %s", rec.Header().Get("Tus-Version"))
	}
	if !strings.Contains(rec.Header().Get("Tus-Extension"), "creation") {
		t.Errorf("expected Tus-Extension to contain creation, got %s", rec.Header().Get("Tus-Extension"))
	}
}

func TestTusQuotaRejection(t *testing.T) {
	_, e, _ := setupTusTestEnvironment(t, 50) // only 50 bytes quota

	metadata := fmt.Sprintf("filename %s", base64.StdEncoding.EncodeToString([]byte("large.bin")))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/upload-tus", nil)
	req.Header.Set("Upload-Length", "100") // 100 bytes > 50 bytes
	req.Header.Set("Upload-Metadata", metadata)
	req.Header.Set("Tus-Resumable", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest && rec.Code != http.StatusForbidden {
		t.Fatalf("expected quota rejection status (413/400/403), got %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestTusResumableUploadFlow(t *testing.T) {
	handler, e, tusTemp := setupTusTestEnvironment(t, 10000)

	filename := "test-doc.txt"
	encodedName := base64.StdEncoding.EncodeToString([]byte(filename))
	totalLength := int64(100)

	// Step 1: POST /upload-tus to initialize upload
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/upload-tus", nil)
	postReq.Header.Set("Upload-Length", fmt.Sprintf("%d", totalLength))
	postReq.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s", encodedName))
	postReq.Header.Set("Tus-Resumable", "1.0.0")
	postRec := httptest.NewRecorder()
	e.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST expected 201 Created, got %d: %s", postRec.Code, postRec.Body.String())
	}
	location := postRec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header in POST response")
	}
	if postRec.Header().Get("Tus-Resumable") != "1.0.0" {
		t.Fatalf("expected Tus-Resumable 1.0.0, got %s", postRec.Header().Get("Tus-Resumable"))
	}

	uploadID := filepath.Base(location)

	// Verify temp files created
	if _, err := os.Stat(filepath.Join(tusTemp, uploadID+".info")); err != nil {
		t.Fatalf("expected .info file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tusTemp, uploadID+".part")); err != nil {
		t.Fatalf("expected .part file to exist: %v", err)
	}

	// Step 2: Upload partial chunk (40 bytes) via PATCH
	chunk1 := bytes.Repeat([]byte("A"), 40)
	patch1Req := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(chunk1))
	patch1Req.Header.Set("Upload-Offset", "0")
	patch1Req.Header.Set("Content-Type", "application/offset+octet-stream")
	patch1Req.Header.Set("Tus-Resumable", "1.0.0")
	patch1Rec := httptest.NewRecorder()
	e.ServeHTTP(patch1Rec, patch1Req)

	if patch1Rec.Code != http.StatusNoContent {
		t.Fatalf("PATCH chunk 1 expected 204 No Content, got %d: %s", patch1Rec.Code, patch1Rec.Body.String())
	}
	if patch1Rec.Header().Get("Upload-Offset") != "40" {
		t.Fatalf("expected Upload-Offset 40, got %s", patch1Rec.Header().Get("Upload-Offset"))
	}

	// Step 3: Query offset via HEAD
	headReq := httptest.NewRequest(http.MethodHead, location, nil)
	headReq.Header.Set("Tus-Resumable", "1.0.0")
	headRec := httptest.NewRecorder()
	e.ServeHTTP(headRec, headReq)

	if headRec.Code != http.StatusOK {
		t.Fatalf("HEAD expected 200 OK, got %d", headRec.Code)
	}
	if headRec.Header().Get("Upload-Offset") != "40" {
		t.Fatalf("HEAD expected Upload-Offset 40, got %s", headRec.Header().Get("Upload-Offset"))
	}
	if headRec.Header().Get("Upload-Length") != "100" {
		t.Fatalf("HEAD expected Upload-Length 100, got %s", headRec.Header().Get("Upload-Length"))
	}

	// Step 4: Test offset mismatch rejection (409 Conflict)
	mismatchReq := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader([]byte("dummy")))
	mismatchReq.Header.Set("Upload-Offset", "0") // wrong offset, should be 40
	mismatchReq.Header.Set("Content-Type", "application/offset+octet-stream")
	mismatchReq.Header.Set("Tus-Resumable", "1.0.0")
	mismatchRec := httptest.NewRecorder()
	e.ServeHTTP(mismatchRec, mismatchReq)

	if mismatchRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on offset mismatch, got %d", mismatchRec.Code)
	}
	if mismatchRec.Header().Get("Upload-Offset") != "40" {
		t.Fatalf("expected current offset 40 in 409 response, got %s", mismatchRec.Header().Get("Upload-Offset"))
	}

	// Step 5: Resume upload to completion (remaining 60 bytes)
	chunk2 := bytes.Repeat([]byte("B"), 60)
	patch2Req := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(chunk2))
	patch2Req.Header.Set("Upload-Offset", "40")
	patch2Req.Header.Set("Content-Type", "application/offset+octet-stream")
	patch2Req.Header.Set("Tus-Resumable", "1.0.0")
	patch2Rec := httptest.NewRecorder()
	e.ServeHTTP(patch2Rec, patch2Req)

	if patch2Rec.Code != http.StatusNoContent {
		t.Fatalf("PATCH chunk 2 expected 204 No Content, got %d: %s", patch2Rec.Code, patch2Rec.Body.String())
	}
	if patch2Rec.Header().Get("Upload-Offset") != "100" {
		t.Fatalf("expected Upload-Offset 100, got %s", patch2Rec.Header().Get("Upload-Offset"))
	}
	uploadedItemID := patch2Rec.Header().Get("Upload-Item-Id")
	if uploadedItemID == "" {
		t.Fatal("expected Upload-Item-Id header on completed upload")
	}

	// Step 6: Verify temp files cleaned up
	if _, err := os.Stat(filepath.Join(tusTemp, uploadID+".info")); !os.IsNotExist(err) {
		t.Errorf("expected .info file to be deleted after finalization, but still exists")
	}
	if _, err := os.Stat(filepath.Join(tusTemp, uploadID+".part")); !os.IsNotExist(err) {
		t.Errorf("expected .part file to be deleted after finalization, but still exists")
	}

	// Step 7: Verify file in database VFS
	var name string
	var sizeBytes int64
	var kind string
	err := handler.service.db.QueryRow(
		`SELECT name, size_bytes, kind FROM my_storage_items WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
		uploadedItemID, "test-user",
	).Scan(&name, &sizeBytes, &kind)
	if err != nil {
		t.Fatalf("failed to query finalized item in db: %v", err)
	}
	if name != filename {
		t.Errorf("expected file name %s, got %s", filename, name)
	}
	if sizeBytes != 100 {
		t.Errorf("expected file size 100, got %d", sizeBytes)
	}
	if kind != "file" {
		t.Errorf("expected kind 'file', got %s", kind)
	}

	// Step 8: Verify quota deducted
	quotaRemaining, err := handler.service.quotaRemaining(context.Background(), handler.service.db.DB, "test-user")
	if err != nil {
		t.Fatalf("failed to check remaining quota: %v", err)
	}
	if quotaRemaining != 10000-100 {
		t.Errorf("expected remaining quota %d, got %d", 10000-100, quotaRemaining)
	}
}

func TestTusDelete(t *testing.T) {
	_, e, tusTemp := setupTusTestEnvironment(t, 10000)

	// Create upload
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/upload-tus", nil)
	postReq.Header.Set("Upload-Length", "50")
	postReq.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s", base64.StdEncoding.EncodeToString([]byte("cancel.bin"))))
	postReq.Header.Set("Tus-Resumable", "1.0.0")
	postRec := httptest.NewRecorder()
	e.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", postRec.Code)
	}
	location := postRec.Header().Get("Location")
	uploadID := filepath.Base(location)

	// DELETE
	delReq := httptest.NewRequest(http.MethodDelete, location, nil)
	delReq.Header.Set("Tus-Resumable", "1.0.0")
	delRec := httptest.NewRecorder()
	e.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE expected 204 No Content, got %d", delRec.Code)
	}

	// Check files deleted
	if _, err := os.Stat(filepath.Join(tusTemp, uploadID+".info")); !os.IsNotExist(err) {
		t.Errorf("expected .info file to be deleted")
	}
	if _, err := os.Stat(filepath.Join(tusTemp, uploadID+".part")); !os.IsNotExist(err) {
		t.Errorf("expected .part file to be deleted")
	}

	// Subsequent HEAD should return 404
	headReq := httptest.NewRequest(http.MethodHead, location, nil)
	headReq.Header.Set("Tus-Resumable", "1.0.0")
	headRec := httptest.NewRecorder()
	e.ServeHTTP(headRec, headReq)

	if headRec.Code != http.StatusNotFound {
		t.Fatalf("HEAD after DELETE expected 404 Not Found, got %d", headRec.Code)
	}
}
