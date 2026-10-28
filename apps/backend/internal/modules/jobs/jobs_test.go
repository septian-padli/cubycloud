package jobs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/sse"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupTestDB(t *testing.T) (*bun.DB, string, storage.StorageDriver) {
	t.Helper()
	dataDir, err := os.MkdirTemp("", "cuby-jobs-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(dataDir, "cubycloud.db")
	db, err := database.Open(context.Background(), dbPath)
	require.NoError(t, err)

	driver := storage.NewStorageDriver(db, dataDir)
	return db, dataDir, driver
}

func seedUserAndStorageNode(t *testing.T, db *bun.DB, userID, nodeID string, capacity int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `
		INSERT OR REPLACE INTO users (id, username, email, password_hash, role, quota_bytes)
		VALUES (?, 'testuser', 'testuser@example.com', 'hash', 'REGULAR', 1073741824)
	`, userID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO storage_nodes (id, name, type, path, capacity_bytes, enabled)
		VALUES (?, 'Local Node', 'LOCAL', '', ?, 1)
	`, nodeID, capacity)
	require.NoError(t, err)
}

func TestQueue_LifecycleAndRecovery(t *testing.T) {
	db, dataDir, _ := setupTestDB(t)
	defer os.RemoveAll(dataDir)
	defer db.Close()

	ctx := context.Background()
	queue := NewQueue(db)

	// 1. Enqueue
	job, err := queue.Enqueue(ctx, TypeCompressZip, "user-1", map[string]string{"foo": "bar"})
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, StatusPending, job.Status)
	assert.Equal(t, 0, job.Progress)

	// 2. Claim
	claimed, err := queue.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, job.ID, claimed.ID)
	assert.Equal(t, StatusProcessing, claimed.Status)
	assert.NotNil(t, claimed.StartedAt)

	// Second claim should return nil (empty queue)
	empty, err := queue.Claim(ctx)
	require.NoError(t, err)
	assert.Nil(t, empty)

	// 3. Update Progress
	err = queue.UpdateProgress(ctx, job.ID, 50)
	require.NoError(t, err)
	j1, err := queue.Get(ctx, job.ID, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 50, j1.Progress)

	// 4. Test Server Restart Recovery on lingering PROCESSING job
	recovered, err := queue.RecoverInterrupted(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), recovered)

	jRec, err := queue.Get(ctx, job.ID, "user-1")
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, jRec.Status)
	require.NotNil(t, jRec.ErrorMessage)
	assert.Contains(t, *jRec.ErrorMessage, "Interrupted by server restart")
}

func TestWorkerPool_Execution(t *testing.T) {
	db, dataDir, _ := setupTestDB(t)
	defer os.RemoveAll(dataDir)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := sse.NewHub()
	queue := NewQueue(db)
	pool := NewWorkerPool(db, queue, hub)

	executed := make(chan string, 1)
	pool.RegisterHandler("dummy_job", func(ctx context.Context, job *database.Job, updateProgress func(int)) (any, error) {
		updateProgress(50)
		executed <- job.ID
		return map[string]string{"result": "ok"}, nil
	})

	pool.Start(ctx)
	defer pool.Stop()

	job, err := queue.Enqueue(ctx, "dummy_job", "user-2", map[string]string{"test": "data"})
	require.NoError(t, err)

	select {
	case id := <-executed:
		assert.Equal(t, job.ID, id)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker execution")
	}

	time.Sleep(100 * time.Millisecond)
	completedJob, err := queue.Get(ctx, job.ID, "user-2")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, completedJob.Status)
	assert.Equal(t, 100, completedJob.Progress)
	require.NotNil(t, completedJob.Result)
	assert.Contains(t, *completedJob.Result, `"result":"ok"`)
}

func TestCompressHandler_MyStorageAndOrganization(t *testing.T) {
	db, dataDir, driver := setupTestDB(t)
	defer os.RemoveAll(dataDir)
	defer db.Close()

	ctx := context.Background()
	userID := "user-compress"
	nodeID := "node-compress"
	seedUserAndStorageNode(t, db, userID, nodeID, 100*1024*1024)

	// Create physical blobs and VFS items in My Storage
	// File 1: /hello.txt
	k1 := "blob-k1"
	err := driver.WriteLocal(ctx, "", k1, strings.NewReader("Hello World!"))
	require.NoError(t, err)
	now := time.Now().UTC()
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		VALUES ('file-1', ?, 'hello.txt', 'file', 12, NULL, ?, ?, ?)
	`, userID, k1, nodeID, now)
	require.NoError(t, err)

	// Folder 1: /Docs
	// File 2: /Docs/report.pdf
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		VALUES ('folder-1', ?, 'Docs', 'folder', 0, NULL, NULL, ?, ?)
	`, userID, nodeID, now)
	require.NoError(t, err)

	k2 := "blob-k2"
	err = driver.WriteLocal(ctx, "", k2, strings.NewReader("PDF Content Data"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		VALUES ('file-2', ?, 'report.pdf', 'file', 16, 'folder-1', ?, ?, ?)
	`, userID, k2, nodeID, now)
	require.NoError(t, err)

	handler := NewCompressHandler(db, driver)

	// Test 1: Compress to ZIP in root folder (TargetParentID = nil)
	payload := CompressZipPayload{
		Scope:          "my_storage",
		UserID:         userID,
		ItemIDs:        []string{"file-1", "folder-1"},
		TargetParentID: nil,
		TargetName:     "Archive.zip",
	}
	payloadBytes, _ := json.Marshal(payload)
	job := &database.Job{
		ID:      ulid.Make().String(),
		Type:    TypeCompressZip,
		UserID:  userID,
		Payload: string(payloadBytes),
	}

	progressUpdates := []int{}
	res, err := handler.Execute(ctx, job, func(p int) {
		progressUpdates = append(progressUpdates, p)
	})
	require.NoError(t, err)
	zipResult, ok := res.(CompressZipResult)
	require.True(t, ok)
	assert.Equal(t, "Archive.zip", zipResult.Name)
	assert.Nil(t, zipResult.ParentID)
	assert.Greater(t, zipResult.SizeBytes, int64(0))

	// Verify database record created
	var count int
	var diskKey string
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*), disk_key FROM my_storage_items WHERE id = ?
	`, zipResult.ItemID).Scan(&count, &diskKey)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify ZIP archive contents
	readCloser, err := driver.ReadLocal(ctx, "", diskKey)
	require.NoError(t, err)
	defer readCloser.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, readCloser)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	entryNames := []string{}
	for _, f := range zr.File {
		entryNames = append(entryNames, f.Name)
	}
	assert.Contains(t, entryNames, "hello.txt")
	assert.Contains(t, entryNames, "Docs/report.pdf")

	// Test 2: Name conflict resolution in the same folder produces "Archive (1).zip"
	job2 := &database.Job{
		ID:      ulid.Make().String(),
		Type:    TypeCompressZip,
		UserID:  userID,
		Payload: string(payloadBytes),
	}
	res2, err := handler.Execute(ctx, job2, nil)
	require.NoError(t, err)
	zipResult2 := res2.(CompressZipResult)
	assert.Equal(t, "Archive (1).zip", zipResult2.Name)

	// Test 3: Action-origin folder persistence: TargetParentID inside folder-1
	payloadInFolder := CompressZipPayload{
		Scope:          "my_storage",
		UserID:         userID,
		ItemIDs:        []string{"file-1"},
		TargetParentID: &zipResult.ItemID, // place inside folder
		TargetName:     "Inside.zip",
	}
	// folder-1 is a folder
	folderID := "folder-1"
	payloadInFolder.TargetParentID = &folderID
	payloadInFolderBytes, _ := json.Marshal(payloadInFolder)
	job3 := &database.Job{
		ID:      ulid.Make().String(),
		Type:    TypeCompressZip,
		UserID:  userID,
		Payload: string(payloadInFolderBytes),
	}
	res3, err := handler.Execute(ctx, job3, nil)
	require.NoError(t, err)
	zipResult3 := res3.(CompressZipResult)
	require.NotNil(t, zipResult3.ParentID)
	assert.Equal(t, folderID, *zipResult3.ParentID)

	// Test 4: Organization Compress
	// Seed organization file
	kOrg := "blob-org-1"
	err = driver.WriteLocal(ctx, "", kOrg, strings.NewReader("Org Shared Content"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		VALUES ('org-file-1', ?, 'org_doc.txt', 'file', 18, NULL, ?, ?, ?)
	`, userID, kOrg, nodeID, now)
	require.NoError(t, err)

	orgPayload := CompressZipPayload{
		Scope:          "organization",
		UserID:         userID,
		ItemIDs:        []string{"org-file-1"},
		TargetParentID: nil,
		TargetName:     "OrgArchive.zip",
	}
	orgPayloadBytes, _ := json.Marshal(orgPayload)
	jobOrg := &database.Job{
		ID:      ulid.Make().String(),
		Type:    TypeCompressZip,
		UserID:  userID,
		Payload: string(orgPayloadBytes),
	}
	resOrg, err := handler.Execute(ctx, jobOrg, nil)
	require.NoError(t, err)
	zipResultOrg := resOrg.(CompressZipResult)
	assert.Equal(t, "OrgArchive.zip", zipResultOrg.Name)
	assert.Equal(t, "organization", zipResultOrg.Scope)

	var orgCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_items WHERE id=?`, zipResultOrg.ItemID).Scan(&orgCount)
	require.NoError(t, err)
	assert.Equal(t, 1, orgCount)
}

func TestJobHandler_GetJobAPI(t *testing.T) {
	db, dataDir, _ := setupTestDB(t)
	defer os.RemoveAll(dataDir)
	defer db.Close()

	ctx := context.Background()
	queue := NewQueue(db)
	job, err := queue.Enqueue(ctx, TypeCompressZip, "user-owner", map[string]string{"foo": "bar"})
	require.NoError(t, err)

	h := NewHandler(queue)
	e := echo.New()

	// 1. Owner gets job -> 200 OK
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+job.ID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(job.ID)
		c.Set("user_id", "user-owner")
		c.Set("role", "USER")

		err := h.GetJob(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), job.ID)
	}

	// 2. Non-owner gets job -> 403 Forbidden
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+job.ID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(job.ID)
		c.Set("user_id", "user-other")
		c.Set("role", "USER")

		err := h.GetJob(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	}

	// 3. Super Admin gets job -> 200 OK
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+job.ID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(job.ID)
		c.Set("user_id", "admin-user")
		c.Set("role", "SUPER_ADMIN")

		err := h.GetJob(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestWorkerPool_DynamicConcurrencyAdjustment(t *testing.T) {
	db, dataDir, _ := setupTestDB(t)
	defer os.RemoveAll(dataDir)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := sse.NewHub()
	queue := NewQueue(db)
	pool := NewWorkerPool(db, queue, hub)
	require.Equal(t, 2, pool.Concurrency())

	releaseJob := make(chan struct{})
	started := make(chan string, 10)

	pool.RegisterHandler("blockable_job", func(ctx context.Context, job *database.Job, updateProgress func(int)) (any, error) {
		started <- job.ID
		select {
		case <-releaseJob:
		case <-ctx.Done():
		}
		return map[string]string{"status": "ok"}, nil
	})

	pool.Start(ctx)
	defer pool.Stop()

	// 1. Concurrency starts at 2: Enqueue 6 jobs
	for i := range 6 {
		_, err := queue.Enqueue(ctx, "blockable_job", "user-1", map[string]int{"idx": i})
		require.NoError(t, err)
	}
	// Only 2 jobs should start
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job 1")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job 2")
	}
	assert.Equal(t, 2, pool.ActiveJobs())

	// Ensure no 3rd job starts yet
	select {
	case id := <-started:
		t.Fatalf("unexpected 3rd job started: %s", id)
	case <-time.After(300 * time.Millisecond):
	}

	// 2. Dynamically increase concurrency: 2 -> 4 (no restart)
	pool.SetConcurrency(4)
	assert.Equal(t, 4, pool.Concurrency())

	// 2 more jobs should now be picked up
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job 3 after scaling up")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job 4 after scaling up")
	}
	assert.Equal(t, 4, pool.ActiveJobs())

	// 3. Dynamically decrease concurrency: 4 -> 1
	// The 4 currently active jobs MUST NOT be cancelled and must complete gracefully.
	pool.SetConcurrency(1)
	assert.Equal(t, 1, pool.Concurrency())
	assert.Equal(t, 4, pool.ActiveJobs())

	// Release all running jobs
	close(releaseJob)

	// Wait for remaining queued jobs (jobs 5 and 6) to run with new concurrency limit 1
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job 5")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job 6")
	}
}
