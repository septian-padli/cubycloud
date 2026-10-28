package jobs_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type mockS3Backend struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func newMockS3Backend() *mockS3Backend {
	return &mockS3Backend{
		objects: make(map[string][]byte),
	}
}

func (m *mockS3Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(key, "/", 2)

	switch r.Method {
	case http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m.objects[key] = data
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		data, exists := m.objects[key]
		if !exists {
			http.Error(w, "NoSuchKey", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case http.MethodHead:
		if len(parts) == 1 || parts[1] == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		data, exists := m.objects[key]
		if !exists {
			http.Error(w, "NoSuchKey", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		delete(m.objects, key)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusOK)
	}
}

func setupTestNodes(t *testing.T) (*database.StorageNode, *database.StorageNode, *database.StorageNode, *database.StorageNode, *mockS3Backend, storage.StorageDriver, *jobs.NodeMigrationHandler, *jobs.Queue, *bun.DB) {
	ctx := context.Background()
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))

	db, err := database.Connect(tempDir)
	require.NoError(t, err)

	storagePath := filepath.Join(tempDir, "storage")
	require.NoError(t, os.MkdirAll(storagePath, 0755))
	keyMaterial, err := storage.InitMasterKey(tempDir, "", "", db)
	require.NoError(t, err)
	driver := storage.NewStorageDriver(db, storagePath)
	s3Backend := newMockS3Backend()
	s3Server := httptest.NewServer(s3Backend)
	t.Cleanup(func() {
		s3Server.Close()
		_ = db.Close()
	})

	// keyMaterial already initialized
	accessEnc, err := storage.EncryptCredential([]byte("mock_access"), keyMaterial)
	require.NoError(t, err)
	secretEnc, err := storage.EncryptCredential([]byte("mock_secret"), keyMaterial)
	require.NoError(t, err)

	localPath1 := filepath.Join(storagePath, "local1")
	localPath2 := filepath.Join(storagePath, "local2")
	require.NoError(t, os.MkdirAll(localPath1, 0755))
	require.NoError(t, os.MkdirAll(localPath2, 0755))

	capBytes := int64(10 * 1024 * 1024 * 1024)

	local1 := &database.StorageNode{
		ID:            "local-node-1",
		Name:          "Local Node 1",
		Type:          "LOCAL",
		Path:          &localPath1,
		CapacityBytes: capBytes,
		Enabled:       true,
		CreatedAt:     time.Now(),
	}
	local2 := &database.StorageNode{
		ID:            "local-node-2",
		Name:          "Local Node 2",
		Type:          "LOCAL",
		Path:          &localPath2,
		CapacityBytes: capBytes,
		Enabled:       true,
		CreatedAt:     time.Now(),
	}
	b1 := "bucket1"
	b2 := "bucket2"
	reg := "us-east-1"
	s3_1 := &database.StorageNode{
		ID:                 "s3-node-1",
		Name:               "S3 Node 1",
		Type:               "S3",
		Endpoint:           &s3Server.URL,
		Bucket:             &b1,
		Region:             &reg,
		AccessKeyEncrypted: accessEnc,
		SecretKeyEncrypted: secretEnc,
		CapacityBytes:      capBytes,
		Enabled:            true,
		CreatedAt:          time.Now(),
	}
	s3_2 := &database.StorageNode{
		ID:                 "s3-node-2",
		Name:               "S3 Node 2",
		Type:               "S3",
		Endpoint:           &s3Server.URL,
		Bucket:             &b2,
		Region:             &reg,
		AccessKeyEncrypted: accessEnc,
		SecretKeyEncrypted: secretEnc,
		CapacityBytes:      capBytes,
		Enabled:            true,
		CreatedAt:          time.Now(),
	}

	for _, n := range []*database.StorageNode{local1, local2, s3_1, s3_2} {
		_, err := db.NewInsert().Model(n).Exec(ctx)
		require.NoError(t, err)
	}

	queue := jobs.NewQueue(db)
	handler := jobs.NewNodeMigrationHandler(db, driver)


	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, role)
		VALUES ('user-1', 'testuser', 'testuser@example.test', 'hash', 'SUPER_ADMIN')`)
	require.NoError(t, err)
	return local1, local2, s3_1, s3_2, s3Backend, driver, handler, queue, db
}

func TestNodeMigrationAllFourCombinations(t *testing.T) {
	ctx := context.Background()
	local1, local2, s3_1, s3_2, s3Backend, driver, handler, queue, db := setupTestNodes(t)

	// 1. Combination: LOCAL -> LOCAL
	t.Run("LOCAL to LOCAL", func(t *testing.T) {
		diskKey := "blob_local_to_local.bin"
		content := []byte("hello-local-to-local-data-stream-verified")
		require.NoError(t, driver.WriteLocal(ctx, *local1.Path, diskKey, bytes.NewReader(content)))

		itemID := ulid.Make().String()
		_, err := db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id, modified_at)
			VALUES (?, 'user-1', 'file1.txt', 'file', ?, ?, ?, CURRENT_TIMESTAMP)`, itemID, len(content), diskKey, local1.ID)
		require.NoError(t, err)

		job, err := queue.Enqueue(ctx, jobs.TypeNodeMigration, "admin-1", jobs.NodeMigrationPayload{
			SourceNodeID: local1.ID,
			TargetNodeID: local2.ID,
		})
		require.NoError(t, err)

		progressCalled := false
		resAny, err := handler.Execute(ctx, job, func(p int) { progressCalled = true })
		require.NoError(t, err)
		require.True(t, progressCalled)

		res := resAny.(*jobs.NodeMigrationResult)
		require.Equal(t, 1, res.MigratedFiles)
		require.Equal(t, 0, res.FailedFiles)
		require.True(t, res.SafeToDisconnect)

		// Destination read-back must succeed and match content
		dstReader, err := driver.ReadLocal(ctx, *local2.Path, diskKey)
		require.NoError(t, err)
		dstBytes, err := io.ReadAll(dstReader)
		dstReader.Close()
		require.NoError(t, err)
		require.Equal(t, content, dstBytes)

		// Zero deletion: Source file must still exist on disk
		srcReader, err := driver.ReadLocal(ctx, *local1.Path, diskKey)
		require.NoError(t, err)
		srcBytes, err := io.ReadAll(srcReader)
		srcReader.Close()
		require.NoError(t, err)
		require.Equal(t, content, srcBytes)

		// ReadBlob resolves transparently to destination
		blobReader, err := storage.ReadBlob(ctx, db, driver, "my_storage_items", diskKey)
		require.NoError(t, err)
		blobBytes, err := io.ReadAll(blobReader)
		_ = blobReader.Close()
		require.NoError(t, err)
		require.Equal(t, content, blobBytes)
	})

	// 2. Combination: LOCAL -> S3
	t.Run("LOCAL to S3", func(t *testing.T) {
		diskKey := "blob_local_to_s3.bin"
		content := []byte("data-moving-from-local-disk-to-s3-bucket")
		require.NoError(t, driver.WriteLocal(ctx, *local2.Path, diskKey, bytes.NewReader(content)))

		itemID := ulid.Make().String()
		_, err := db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id, modified_at)
			VALUES (?, 'user-1', 'file2.txt', 'file', ?, ?, ?, CURRENT_TIMESTAMP)`, itemID, len(content), diskKey, local2.ID)
		require.NoError(t, err)

		job, err := queue.Enqueue(ctx, jobs.TypeNodeMigration, "admin-1", jobs.NodeMigrationPayload{
			SourceNodeID: local2.ID,
			TargetNodeID: s3_1.ID,
		})
		require.NoError(t, err)

		resAny, err := handler.Execute(ctx, job, func(p int) {})
		require.NoError(t, err)
		res := resAny.(*jobs.NodeMigrationResult)
		require.True(t, res.SafeToDisconnect)
		require.Equal(t, 2, res.MigratedFiles)
		// S3 destination must hold object
		s3Backend.mu.RLock()
		s3Data := s3Backend.objects["bucket1/"+diskKey]
		s3Backend.mu.RUnlock()
		require.Equal(t, content, s3Data)

		// Source file remains untouched
		srcReader, err := driver.ReadLocal(ctx, *local2.Path, diskKey)
		require.NoError(t, err)
		_ = srcReader.Close()
	})

	// 3. Combination: S3 -> LOCAL
	t.Run("S3 to LOCAL", func(t *testing.T) {
		diskKey := "blob_s3_to_local.bin"
		content := []byte("data-moving-from-s3-bucket-to-local-disk")
		s3Backend.mu.Lock()
		s3Backend.objects["bucket1/"+diskKey] = content
		s3Backend.mu.Unlock()

		itemID := ulid.Make().String()
		_, err := db.Exec(`INSERT INTO organization_items (id, created_by, name, kind, size_bytes, disk_key, storage_node_id, modified_at)
			VALUES (?, 'user-1', 'orgfile.txt', 'file', ?, ?, ?, CURRENT_TIMESTAMP)`, itemID, len(content), diskKey, s3_1.ID)
		require.NoError(t, err)

		// Ensure local2 is enabled for write
		_, _ = db.Exec(`UPDATE storage_nodes SET enabled = 1 WHERE id = ?`, local2.ID)

		job, err := queue.Enqueue(ctx, jobs.TypeNodeMigration, "admin-1", jobs.NodeMigrationPayload{
			SourceNodeID: s3_1.ID,
			TargetNodeID: local2.ID,
		})
		require.NoError(t, err)

		resAny, err := handler.Execute(ctx, job, func(p int) {})
		require.NoError(t, err)
		res := resAny.(*jobs.NodeMigrationResult)
		require.True(t, res.SafeToDisconnect)

		// Destination file verified on local disk
		dstReader, err := driver.ReadLocal(ctx, *local2.Path, diskKey)
		require.NoError(t, err)
		dstBytes, err := io.ReadAll(dstReader)
		dstReader.Close()
		require.NoError(t, err)
		require.Equal(t, content, dstBytes)

		// Zero deletion: S3 object must still exist in mock
		s3Backend.mu.RLock()
		s3Obj := s3Backend.objects["bucket1/"+diskKey]
		s3Backend.mu.RUnlock()
		require.Equal(t, content, s3Obj)
	})

	// 4. Combination: S3 -> S3
	t.Run("S3 to S3", func(t *testing.T) {
		diskKey := "blob_s3_to_s3.bin"
		content := []byte("data-streaming-between-two-cloud-buckets")
		s3Backend.mu.Lock()
		s3Backend.objects["bucket1/"+diskKey] = content
		s3Backend.mu.Unlock()

		itemID := ulid.Make().String()
		_, err := db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id, modified_at)
			VALUES (?, 'user-1', 's3file.txt', 'file', ?, ?, ?, CURRENT_TIMESTAMP)`, itemID, len(content), diskKey, s3_1.ID)
		require.NoError(t, err)

		job, err := queue.Enqueue(ctx, jobs.TypeNodeMigration, "admin-1", jobs.NodeMigrationPayload{
			SourceNodeID: s3_1.ID,
			TargetNodeID: s3_2.ID,
		})
		require.NoError(t, err)

		resAny, err := handler.Execute(ctx, job, func(p int) {})
		require.NoError(t, err)
		res := resAny.(*jobs.NodeMigrationResult)
		require.True(t, res.SafeToDisconnect)

		// S3_2 destination must have object
		s3Backend.mu.RLock()
		s3ObjDst := s3Backend.objects["bucket2/"+diskKey]
		s3Backend.mu.RUnlock()
		require.Equal(t, content, s3ObjDst)

		// Zero deletion: S3_1 object remains intact
		s3Backend.mu.RLock()
		s3ObjSrc := s3Backend.objects["bucket1/"+diskKey]
		s3Backend.mu.RUnlock()
		require.Equal(t, content, s3ObjSrc)
	})

	// 5. Failure scenario: Target failure retains source readability and does NOT declare node safe to disconnect
	t.Run("Failure retains source access", func(t *testing.T) {
		diskKey := "blob_fail_retains_source.bin"
		content := []byte("important-user-file-that-must-not-be-lost")

		// Re-enable local1
		_, _ = db.Exec(`UPDATE storage_nodes SET enabled = 1 WHERE id = ?`, local1.ID)
		require.NoError(t, driver.WriteLocal(ctx, *local1.Path, diskKey, bytes.NewReader(content)))

		itemID := ulid.Make().String()
		_, err := db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id, modified_at)
			VALUES (?, 'user-1', 'important.txt', 'file', ?, ?, ?, CURRENT_TIMESTAMP)`, itemID, len(content), diskKey, local1.ID)
		require.NoError(t, err)

		// Point target to a non-existent or invalid directory to force write failure
		invalidTargetID := "invalid-node"
		invalidPath := "/invalid/nonexistent/directory"
		_, err = db.Exec(`INSERT INTO storage_nodes (id, name, type, path, capacity_bytes, enabled)
			VALUES (?, 'Invalid Node', 'LOCAL', ?, 1000000000, 1)`, invalidTargetID, invalidPath)
		require.NoError(t, err)

		job, err := queue.Enqueue(ctx, jobs.TypeNodeMigration, "admin-1", jobs.NodeMigrationPayload{
			SourceNodeID: local1.ID,
			TargetNodeID: invalidTargetID,
		})
		require.NoError(t, err)

		resAny, err := handler.Execute(ctx, job, func(p int) {})
		require.Error(t, err, "migration should return error on failure")
		if resAny != nil {
			res := resAny.(*jobs.NodeMigrationResult)
			require.False(t, res.SafeToDisconnect, "must NOT be safe to disconnect on failure")
			require.Greater(t, res.FailedFiles, 0)
		}

		// Verify database pointer still points to source node
		var currentNodeID string
		row := db.QueryRow(`SELECT storage_node_id FROM my_storage_items WHERE id = ?`, itemID)
		require.NoError(t, row.Scan(&currentNodeID))
		require.Equal(t, local1.ID, currentNodeID, "database reference must remain on source node")

		// Verify source file is still readable and intact
		srcReader, err := driver.ReadLocal(ctx, *local1.Path, diskKey)
		require.NoError(t, err, "source file must remain readable")
		srcBytes, err := io.ReadAll(srcReader)
		srcReader.Close()
		require.NoError(t, err)
		require.Equal(t, content, srcBytes)
	})
}
