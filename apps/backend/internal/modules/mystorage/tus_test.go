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
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

type gatedTusReader struct {
	started chan struct{}
	release chan struct{}
	data    *bytes.Reader
	once    sync.Once
}

func (r *gatedTusReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		if r.started != nil {
			close(r.started)
			<-r.release
		}
	})
	return r.data.Read(p)
}

func (r *gatedTusReader) Close() error { return nil }

func tusCreateForTest(t *testing.T, e *echo.Echo, length int) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/upload-tus", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", fmt.Sprint(length))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create upload: %d %s", rec.Code, rec.Body.String())
	}
	return rec.Header().Get("Location")
}

func tusPatchForTest(e *echo.Echo, location, offset string, body *gatedTusReader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, location, body)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Offset", offset)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestTusIndependentUploadsDoNotBlockDuringChunkWrite(t *testing.T) {
	_, e, _ := setupTusTestEnvironment(t, 10000)
	slow := tusCreateForTest(t, e, 2)
	fast := tusCreateForTest(t, e, 2)
	body := &gatedTusReader{started: make(chan struct{}), release: make(chan struct{}), data: bytes.NewReader([]byte("a"))}
	defer func() {
		select {
		case <-body.release:
		default:
			close(body.release)
		}
	}()
	slowDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { slowDone <- tusPatchForTest(e, slow, "0", body) }()
	select {
	case <-body.started:
	case <-time.After(5 * time.Second):
		t.Fatal("first PATCH did not begin reading its chunk")
	}
	fastDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		fastDone <- tusPatchForTest(e, fast, "0", &gatedTusReader{data: bytes.NewReader([]byte("b"))})
	}()
	select {
	case rec := <-fastDone:
		if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Offset") != "1" {
			t.Fatalf("independent PATCH: %d offset=%q body=%s", rec.Code, rec.Header().Get("Upload-Offset"), rec.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("independent PATCH blocked behind another upload's chunk")
	}
	close(body.release)
	if rec := <-slowDone; rec.Code != http.StatusNoContent {
		t.Fatalf("first PATCH: %d %s", rec.Code, rec.Body.String())
	}
}

func TestTusSameUploadPatchAndDeleteWaitForChunk(t *testing.T) {
	_, e, dir := setupTusTestEnvironment(t, 10000)
	location := tusCreateForTest(t, e, 4)
	body := &gatedTusReader{started: make(chan struct{}), release: make(chan struct{}), data: bytes.NewReader([]byte("ab"))}
	defer func() {
		select {
		case <-body.release:
		default:
			close(body.release)
		}
	}()
	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- tusPatchForTest(e, location, "0", body) }()
	select {
	case <-body.started:
	case <-time.After(5 * time.Second):
		t.Fatal("first PATCH did not begin reading its chunk")
	}
	secondDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		secondDone <- tusPatchForTest(e, location, "0", &gatedTusReader{data: bytes.NewReader([]byte("xy"))})
	}()
	select {
	case rec := <-secondDone:
		t.Fatalf("same-upload PATCH completed while first chunk in flight: %d", rec.Code)
	case <-time.After(100 * time.Millisecond):
	}
	close(body.release)
	if rec := <-firstDone; rec.Code != http.StatusNoContent {
		t.Fatalf("first PATCH: %d %s", rec.Code, rec.Body.String())
	}
	if rec := <-secondDone; rec.Code != http.StatusConflict || rec.Header().Get("Upload-Offset") != "2" {
		t.Fatalf("stale PATCH: %d offset=%q", rec.Code, rec.Header().Get("Upload-Offset"))
	}

	body = &gatedTusReader{started: make(chan struct{}), release: make(chan struct{}), data: bytes.NewReader([]byte("c"))}
	go func() { firstDone <- tusPatchForTest(e, location, "2", body) }()
	select {
	case <-body.started:
	case <-time.After(5 * time.Second):
		t.Fatal("next PATCH did not begin reading its chunk")
	}
	deleteDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodDelete, location, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		deleteDone <- rec
	}()
	select {
	case rec := <-deleteDone:
		t.Fatalf("DELETE completed during PATCH: %d", rec.Code)
	case <-time.After(100 * time.Millisecond):
	}
	close(body.release)
	if rec := <-firstDone; rec.Code != http.StatusNoContent {
		t.Fatalf("second chunk: %d %s", rec.Code, rec.Body.String())
	}
	if rec := <-deleteDone; rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Base(location)+".part")); !os.IsNotExist(err) {
		t.Fatalf("deleted upload still has a part file: %v", err)
	}
}

func TestTusDefaultDirectoryResumesAfterHandlerRestart(t *testing.T) {
	handler, e, oldDir := setupTusTestEnvironment(t, 10000)
	dataDir := filepath.Dir(oldDir)
	t.Setenv("TEMP_TUS_PATH", "")
	t.Setenv("TUS_TEMP_PATH", "")
	t.Setenv("DATA_DIR_PATH", dataDir)
	location := tusCreateForTest(t, e, 4)
	expectedDir := filepath.Join(dataDir, "temp", "tus")
	if _, err := os.Stat(filepath.Join(expectedDir, filepath.Base(location)+".info")); err != nil {
		t.Fatalf("TUS metadata must be on retained data volume: %v", err)
	}
	rec := tusPatchForTest(e, location, "0", &gatedTusReader{data: bytes.NewReader([]byte("ab"))})
	if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Offset") != "2" {
		t.Fatalf("initial PATCH: %d offset=%q", rec.Code, rec.Header().Get("Upload-Offset"))
	}
	// A fresh handler must recover the offset from files on the retained volume.
	restarted := echo.New()
	g := restarted.Group("/api/v1/drive/my-storage")
	g.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", "test-user")
			return next(c)
		}
	})
	// Reconnect to the database on the retained volume, as a restarted process does.
	if err := handler.service.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := NewHandler(NewService(db, &uploadLimitDriver{capacity: 1000000, free: 900000}))
	h.RegisterTusRoutes(g)
	head := httptest.NewRecorder()
	restarted.ServeHTTP(head, httptest.NewRequest(http.MethodHead, location, nil))
	if head.Code != http.StatusOK || head.Header().Get("Upload-Offset") != "2" {
		t.Fatalf("restart HEAD: %d offset=%q", head.Code, head.Header().Get("Upload-Offset"))
	}
	rec = tusPatchForTest(restarted, location, "2", &gatedTusReader{data: bytes.NewReader([]byte("cd"))})
	if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Offset") != "4" {
		t.Fatalf("resume PATCH: %d offset=%q body=%s", rec.Code, rec.Header().Get("Upload-Offset"), rec.Body.String())
	}
}

func TestTusLockRefcountAndNoMemoryLeak(t *testing.T) {
	const numUploads = 5
	const workersPerUpload = 30

	type uploadTracker struct {
		active int
	}
	trackers := make(map[string]*uploadTracker)
	for i := range numUploads {
		trackers[fmt.Sprintf("leak-test-upload-%d", i)] = &uploadTracker{}
	}

	var wg sync.WaitGroup
	var violated int32

	for id, tr := range trackers {
		for range workersPerUpload {
			wg.Add(1)
			go func(uploadID string, tracker *uploadTracker) {
				defer wg.Done()
				lock := lockTusUpload(uploadID)
				defer unlockTusUpload(uploadID, lock)

				tracker.active++
				if tracker.active != 1 {
					atomic.AddInt32(&violated, 1)
				}
				time.Sleep(50 * time.Microsecond)
				tracker.active--
			}(id, tr)
		}
	}

	wg.Wait()

	if atomic.LoadInt32(&violated) > 0 {
		t.Fatalf("mutual exclusion violated %d times across concurrent workers", violated)
	}

	tusLocks.mu.Lock()
	remainingLocks := len(tusLocks.byID)
	tusLocks.mu.Unlock()

	if remainingLocks != 0 {
		t.Fatalf("expected 0 remaining locks in tusLocks.byID, got %d (memory leak detected)", remainingLocks)
	}
}

func TestTusConcurrentIndependentUploadsMultiple(t *testing.T) {
	_, e, _ := setupTusTestEnvironment(t, 50000)
	const numUploads = 5
	locations := make([]string, numUploads)
	readers := make([]*gatedTusReader, numUploads)
	doneChans := make([]chan *httptest.ResponseRecorder, numUploads)

	for i := range numUploads {
		locations[i] = tusCreateForTest(t, e, 10)
		readers[i] = &gatedTusReader{
			started: make(chan struct{}),
			release: make(chan struct{}),
			data:    bytes.NewReader([]byte("chunk")),
		}
		doneChans[i] = make(chan *httptest.ResponseRecorder, 1)
	}

	defer func() {
		for _, r := range readers {
			select {
			case <-r.release:
			default:
				close(r.release)
			}
		}
	}()

	// Start all chunk writes concurrently
	for i := range numUploads {
		go func(idx int) {
			doneChans[idx] <- tusPatchForTest(e, locations[idx], "0", readers[idx])
		}(i)
	}

	// All numUploads MUST reach their Read() simultaneously without waiting for others
	for i := range numUploads {
		select {
		case <-readers[i].started:
		case <-time.After(5 * time.Second):
			t.Fatalf("upload %d chunk write blocked or timed out; global lock detected", i)
		}
	}

	// Release all readers to complete writes
	for i := range numUploads {
		close(readers[i].release)
	}

	// All uploads should complete with 204 No Content and offset 5
	for i := range numUploads {
		select {
		case rec := <-doneChans[i]:
			if rec.Code != http.StatusNoContent || rec.Header().Get("Upload-Offset") != "5" {
				t.Fatalf("upload %d: code=%d offset=%s", i, rec.Code, rec.Header().Get("Upload-Offset"))
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("upload %d did not complete", i)
		}
	}

	// Verify no memory leak in tusLocks
	tusLocks.mu.Lock()
	count := len(tusLocks.byID)
	tusLocks.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 remaining locks, got %d", count)
	}
}

func TestTusLocksCleanedUpAfterLifecycle(t *testing.T) {
	_, e, _ := setupTusTestEnvironment(t, 10000)

	// 1. Full upload lifecycle: POST -> PATCH (part) -> PATCH (final)
	loc := tusCreateForTest(t, e, 4)
	uploadID := filepath.Base(loc)

	tusLocks.mu.Lock()
	if _, ok := tusLocks.byID[uploadID]; ok {
		t.Errorf("lock for %s should not exist while idle", uploadID)
	}
	tusLocks.mu.Unlock()

	rec := tusPatchForTest(e, loc, "0", &gatedTusReader{data: bytes.NewReader([]byte("ab"))})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("patch 1: %d", rec.Code)
	}

	tusLocks.mu.Lock()
	if _, ok := tusLocks.byID[uploadID]; ok {
		t.Errorf("lock for %s should not exist after patch 1 completes", uploadID)
	}
	tusLocks.mu.Unlock()

	rec = tusPatchForTest(e, loc, "2", &gatedTusReader{data: bytes.NewReader([]byte("cd"))})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("patch 2: %d", rec.Code)
	}

	tusLocks.mu.Lock()
	if _, ok := tusLocks.byID[uploadID]; ok {
		t.Errorf("lock for %s should not exist after upload finalized", uploadID)
	}
	tusLocks.mu.Unlock()

	// 2. Delete flow: POST -> DELETE
	locDel := tusCreateForTest(t, e, 4)
	uploadIDDel := filepath.Base(locDel)

	delReq := httptest.NewRequest(http.MethodDelete, locDel, nil)
	delReq.Header.Set("Tus-Resumable", "1.0.0")
	delRec := httptest.NewRecorder()
	e.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", delRec.Code)
	}

	tusLocks.mu.Lock()
	if _, ok := tusLocks.byID[uploadIDDel]; ok {
		t.Errorf("lock for %s should not exist after DELETE", uploadIDDel)
	}
	tusLocks.mu.Unlock()

	// 3. Non-existent upload requests
	nonExistentLoc := "/api/v1/drive/my-storage/upload-tus/non-existent-id"
	headReq := httptest.NewRequest(http.MethodHead, nonExistentLoc, nil)
	headReq.Header.Set("Tus-Resumable", "1.0.0")
	headRec := httptest.NewRecorder()
	e.ServeHTTP(headRec, headReq)
	if headRec.Code != http.StatusNotFound {
		t.Fatalf("head non-existent: %d", headRec.Code)
	}

	tusLocks.mu.Lock()
	if _, ok := tusLocks.byID["non-existent-id"]; ok {
		t.Errorf("lock for non-existent-id should not remain after 404")
	}
	tusLocks.mu.Unlock()
}
