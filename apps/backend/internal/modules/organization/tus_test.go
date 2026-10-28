package organization_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

type orgTusEnv struct {
	handler *organization.Handler
	e       *echo.Echo
	db      *bun.DB
	orgTemp string
	current atomic.Value // string
}

func setupOrgTusEnv(t *testing.T, quotaBytes int64) *orgTusEnv {
	t.Helper()
	tempDir := t.TempDir()
	orgTemp := filepath.Join(tempDir, "tus_temp", "organization")
	if err := os.MkdirAll(filepath.Join(tempDir, "db"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "storage", "default"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(orgTemp, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEMP_TUS_PATH", filepath.Join(tempDir, "tus_temp"))

	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"org-user", "orguser", "org@example.com", "hash", "REGULAR", quotaBytes)
	if err != nil {
		t.Fatal(err)
	}

	nodePath := filepath.Join(tempDir, "storage", "default")
	_, err = db.Exec(`INSERT INTO storage_nodes(id, type, path, capacity_bytes, enabled, created_at) VALUES(?, 'LOCAL', ?, ?, 1, ?)`,
		ulid.Make().String(), nodePath, int64(100*1024*1024), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	driver := storage.NewStorageDriver(db, filepath.Join(tempDir, "storage"))
	handler := organization.NewHandler(organization.NewService(db, driver))

	e := echo.New()
	env := &orgTusEnv{handler: handler, e: e, db: db, orgTemp: orgTemp}
	env.current.Store("org-user")

	g := e.Group("/api/v1/drive/organization")
	g.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", env.current.Load().(string))
			return next(c)
		}
	})
	handler.RegisterTusRoutes(g)

	return env
}

func orgTusCreateForTest(t *testing.T, e *echo.Echo, length int, meta map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/organization/upload-tus", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", fmt.Sprint(length))
	if encoded := encodeOrgTusMetadata(meta); encoded != "" {
		req.Header.Set("Upload-Metadata", encoded)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create upload: %d %s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/api/v1/drive/organization/upload-tus/") {
		t.Fatalf("unexpected Location %q", location)
	}
	return location
}

func encodeOrgTusMetadata(meta map[string]string) string {
	parts := make([]string, 0, len(meta))
	for key, val := range meta {
		if val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", key, base64.StdEncoding.EncodeToString([]byte(val))))
	}
	return strings.Join(parts, ",")
}

func orgTusPatch(e *echo.Echo, location, offset string, body []byte, checksumHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(body))
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Offset", offset)
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	if checksumHeader != "" {
		req.Header.Set("Upload-Checksum", checksumHeader)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func orgTusHead(e *echo.Echo, location string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodHead, location, nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestOrgTusResumableUploadFlow(t *testing.T) {
	env := setupOrgTusEnv(t, 10000)
	location := orgTusCreateForTest(t, env.e, 10, map[string]string{"filename": "org-doc.txt"})
	uploadID := filepath.Base(location)

	rec := orgTusPatch(env.e, location, "0", []byte("chunk"), "")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Offset") != "5" {
		t.Fatalf("PATCH chunk 1: %d offset=%q body=%s", rec.Code, rec.Header().Get("Upload-Offset"), rec.Body.String())
	}

	head := orgTusHead(env.e, location)
	if head.Code != http.StatusOK || head.Header().Get("Upload-Offset") != "5" {
		t.Fatalf("HEAD: %d offset=%q", head.Code, head.Header().Get("Upload-Offset"))
	}

	mismatch := orgTusPatch(env.e, location, "0", []byte("xx"), "")
	if mismatch.Code != http.StatusConflict || mismatch.Header().Get("Upload-Offset") != "5" {
		t.Fatalf("stale PATCH: %d offset=%q", mismatch.Code, mismatch.Header().Get("Upload-Offset"))
	}

	final := orgTusPatch(env.e, location, "5", []byte("12345"), "")
	if final.Code != http.StatusNoContent || final.Header().Get("Upload-Offset") != "10" {
		t.Fatalf("final PATCH: %d offset=%q body=%s", final.Code, final.Header().Get("Upload-Offset"), final.Body.String())
	}
	itemID := final.Header().Get("Upload-Item-Id")
	if itemID == "" {
		t.Fatal("expected Upload-Item-Id on finalized upload")
	}

	// Regular session state is removed; only the durable done record remains.
	if _, err := os.Stat(filepath.Join(env.orgTemp, uploadID+".part")); !os.IsNotExist(err) {
		t.Fatalf("part file must be removed after finalization: %v", err)
	}

	// The finalized file must exist in organization_items, without user-quota
	// enforcement (org quota budget here is 10000 but the item is stored
	// regardless of per-user accounting).
	var name string
	var kind string
	var createdBy string
	err := env.db.QueryRow(
		`SELECT name, kind, created_by FROM organization_items WHERE id=? AND deleted_at IS NULL`,
		itemID,
	).Scan(&name, &kind, &createdBy)
	if err != nil {
		t.Fatalf("finalized item missing from organization_items: %v", err)
	}
	if name != "org-doc.txt" || kind != "file" || createdBy != "org-user" {
		t.Fatalf("unexpected item: name=%q kind=%q created_by=%q", name, kind, createdBy)
	}
}

func TestOrgTusSkipsUserQuota(t *testing.T) {
	// Tiny quota: My Storage would reject this; Organization must not.
	env := setupOrgTusEnv(t, 10)
	location := orgTusCreateForTest(t, env.e, 1000, map[string]string{"filename": "big.bin"})
	rec := orgTusPatch(env.e, location, "0", []byte(strings.Repeat("x", 100)), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("patch despite tiny user quota: %d %s", rec.Code, rec.Body.String())
	}
	rec = orgTusPatch(env.e, location, "100", []byte(strings.Repeat("y", 900)), "")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Item-Id") == "" {
		t.Fatalf("finalize despite tiny user quota: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOrgTusFinalizeIsIdempotentAfterLostResponse(t *testing.T) {
	env := setupOrgTusEnv(t, 10000)
	location := orgTusCreateForTest(t, env.e, 4, map[string]string{"filename": "keep.txt"})
	orgTusPatch(env.e, location, "0", []byte("ab"), "")
	final := orgTusPatch(env.e, location, "2", []byte("cd"), "")
	if final.Code != http.StatusNoContent {
		t.Fatalf("final PATCH: %d %s", final.Code, final.Body.String())
	}
	itemID := final.Header().Get("Upload-Item-Id")
	if itemID == "" {
		t.Fatal("expected Upload-Item-Id")
	}

	// Client lost the final response and replays the last chunk.
	retry := orgTusPatch(env.e, location, "4", []byte("cd"), "")
	if retry.Code != http.StatusNoContent {
		t.Fatalf("retried final PATCH: %d %s", retry.Code, retry.Body.String())
	}
	if echoID := retry.Header().Get("Upload-Item-Id"); echoID != itemID {
		t.Fatalf("retry echoed different item: %q vs %q", echoID, itemID)
	}

	head := orgTusHead(env.e, location)
	if head.Code != http.StatusOK || head.Header().Get("Upload-Offset") != "4" || head.Header().Get("Upload-Item-Id") != itemID {
		t.Fatalf("HEAD after finalize: %d offset=%q item=%q", head.Code, head.Header().Get("Upload-Offset"), head.Header().Get("Upload-Item-Id"))
	}

	var count int
	if err := env.db.QueryRow(
		`SELECT COUNT(*) FROM organization_items WHERE created_by='org-user' AND deleted_at IS NULL`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 finalized file, got %d", count)
	}
}

func TestOrgTusDigestVerification(t *testing.T) {
	env := setupOrgTusEnv(t, 10000)
	content := []byte("hash-me")
	sum := sha256.Sum256(content)
	checksum := "sha256 " + base64.StdEncoding.EncodeToString(sum[:])

	location := orgTusCreateForTest(t, env.e, 7, nil)
	rec := orgTusPatch(env.e, location, "0", content, checksum)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Item-Id") == "" {
		t.Fatalf("checksum-matching finalize: %d %s", rec.Code, rec.Body.String())
	}

	// Corrupt content with a wrong checksum must never create the file.
	wrongSum := sha256.Sum256([]byte("other content"))
	badChecksum := "sha256 " + base64.StdEncoding.EncodeToString(wrongSum[:])
	badLoc := orgTusCreateForTest(t, env.e, 7, nil)
	badRec := orgTusPatch(env.e, badLoc, "0", content, badChecksum)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on checksum mismatch, got %d", badRec.Code)
	}
	var count int
	if err := env.db.QueryRow(
		`SELECT COUNT(*) FROM organization_items WHERE created_by='org-user' AND deleted_at IS NULL`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("checksum mismatch must not create a file, got %d rows", count)
	}
	if _, err := os.Stat(filepath.Join(env.orgTemp, filepath.Base(badLoc)+".part")); !os.IsNotExist(err) {
		t.Fatalf("rejected upload must be dropped")
	}
}

func TestOrgTusCreatorIsolation(t *testing.T) {
	env := setupOrgTusEnv(t, 10000)
	location := orgTusCreateForTest(t, env.e, 4, map[string]string{"filename": "a.txt"})

	// Another org member must not see or extend someone else's session.
	env.current.Store("other-user")
	head := orgTusHead(env.e, location)
	if head.Code != http.StatusNotFound {
		t.Fatalf("HEAD by other user: %d (expected 404)", head.Code)
	}
	rec := orgTusPatch(env.e, location, "0", []byte("ab"), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PATCH by other user: %d (expected 404)", rec.Code)
	}

	env.current.Store("org-user")
	rec = orgTusPatch(env.e, location, "0", []byte("ab"), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PATCH by creator: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOrgTusDeletedDestinationBlocksFinalizationAndCanRetry(t *testing.T) {
	env := setupOrgTusEnv(t, 10000)
	dataDir := filepath.Dir(filepath.Dir(env.orgTemp))
	driver := storage.NewStorageDriver(env.db, filepath.Join(dataDir, "storage"))
	folder, _, err := organization.NewService(env.db, driver).CreateFolder(
		context.Background(),
		"org-user",
		"TUS destination",
		nil,
		false,
	)
	if err != nil {
		t.Fatalf("create destination: %v", err)
	}
	location := orgTusCreateForTest(t, env.e, 4, map[string]string{
		"filename":  "nested.txt",
		"parent_id": folder.ID,
	})
	if _, err := env.db.Exec(`UPDATE organization_items SET deleted_at=CURRENT_TIMESTAMP WHERE id=?`, folder.ID); err != nil {
		t.Fatalf("delete destination: %v", err)
	}

	failed := orgTusPatch(env.e, location, "0", []byte("data"), "")
	if failed.Code != http.StatusNotFound {
		t.Fatalf("finalize into deleted destination: status %d, want 404: %s", failed.Code, failed.Body.String())
	}
	head := orgTusHead(env.e, location)
	if head.Code != http.StatusOK || head.Header().Get("Upload-Offset") != "4" {
		t.Fatalf("session after deleted destination: status %d, offset %q", head.Code, head.Header().Get("Upload-Offset"))
	}
	var count int
	if err := env.db.QueryRow(
		`SELECT COUNT(*) FROM organization_items WHERE name='nested.txt' AND kind='file' AND deleted_at IS NULL`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("file was created under a deleted destination: count=%d", count)
	}

	if _, err := env.db.Exec(`UPDATE organization_items SET deleted_at=NULL WHERE id=?`, folder.ID); err != nil {
		t.Fatalf("restore destination: %v", err)
	}
	retry := orgTusPatch(env.e, location, "4", nil, "")
	if retry.Code != http.StatusNoContent || retry.Header().Get("Upload-Item-Id") == "" {
		t.Fatalf("retry after restoring destination: status %d, item %q: %s", retry.Code, retry.Header().Get("Upload-Item-Id"), retry.Body.String())
	}
	if err := env.db.QueryRow(
		`SELECT COUNT(*) FROM organization_items WHERE name='nested.txt' AND kind='file' AND deleted_at IS NULL`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retry should create exactly one file, got %d", count)
	}
}

func TestOrgTusCleanupExpiredUploads(t *testing.T) {
	env := setupOrgTusEnv(t, 10000)
	orgTusTemp := env.orgTemp

	fresh := filepath.Base(orgTusCreateForTest(t, env.e, 4, nil))
	if rec := orgTusPatch(env.e, "/api/v1/drive/organization/upload-tus/"+fresh, "0", []byte("ab"), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("fresh patch: %d", rec.Code)
	}

	expired := filepath.Base(orgTusCreateForTest(t, env.e, 4, nil))
	if rec := orgTusPatch(env.e, "/api/v1/drive/organization/upload-tus/"+expired, "0", []byte("cd"), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("expired patch: %d", rec.Code)
	}
	old := time.Now().Add(-25 * time.Hour)
	for _, suffix := range []string{".info", ".part", ".hash"} {
		path := filepath.Join(orgTusTemp, expired+suffix)
		if _, err := os.Stat(path); err == nil {
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Unknown files must never be touched.
	notes := filepath.Join(orgTusTemp, "notes.txt")
	if err := os.WriteFile(notes, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := organization.CleanupExpiredTusUploads(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(orgTusTemp, expired+".part")); !os.IsNotExist(err) {
		t.Fatalf("expired part must be swept")
	}
	if _, err := os.Stat(filepath.Join(orgTusTemp, expired+".info")); !os.IsNotExist(err) {
		t.Fatalf("expired info must be swept")
	}
	if _, err := os.Stat(filepath.Join(orgTusTemp, fresh+".part")); err != nil {
		t.Fatalf("fresh upload must be retained: %v", err)
	}
	if _, err := os.Stat(notes); err != nil {
		t.Fatalf("unrecognized files must never be swept: %v", err)
	}
	head := orgTusHead(env.e, "/api/v1/drive/organization/upload-tus/"+fresh)
	if head.Code != http.StatusOK || head.Header().Get("Upload-Offset") != "2" {
		t.Fatalf("fresh upload must still be resumable: %d", head.Code)
	}
}
