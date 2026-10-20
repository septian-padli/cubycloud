package admin_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/admin"
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

// TestS3NodeLifecycleAndRestartPersistence verifies:
// 1. Adding an S3-compatible node encrypts credentials and validates via HeadBucket.
// 2. Uploading a file places it into the S3 bucket.
// 3. Simulating a full backend shutdown and restart (closing DB & driver, re-opening new instances).
// 4. Reading/downloading the file after restart succeeds with byte-identical content.
// 5. Purging the file from trash deletes the physical object from S3.
func TestS3NodeLifecycleAndRestartPersistence(t *testing.T) {
	// Skip if local MinIO is not running
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:19000/minio/health/live")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("MinIO dev container not running on localhost:19000; skipping live MinIO integration test")
		return
	}
	resp.Body.Close()

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "cubycloud-s3-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	storagePath := filepath.Join(tempDir, "storage")
	require.NoError(t, os.MkdirAll(storagePath, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))
	// 1. Initial Start: Open database and storage driver
	db, err := database.Connect(tempDir)
	require.NoError(t, err)

	driver := storage.NewStorageDriver(db, storagePath)

	// Create test user
	userID := ulid.Make().String()
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, username, email, password_hash, role, quota_mode, quota_bytes, created_at, updated_at)
		VALUES (?, 's3tester', 's3tester@example.test', 'hash', 'SUPER_ADMIN', 'UNLIMITED', -1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID)
	require.NoError(t, err)

	// Create Admin & MyStorage services
	adminSvc := admin.NewService(db, driver, tempDir)
	myStorageSvc := mystorage.NewService(db, driver)
	// 2. Create S3 Node via Admin Service
	nodeName := fmt.Sprintf("MinIO Integration %d", time.Now().UnixNano())
	cloudNode, err := adminSvc.CreateCloudNode(ctx, &admin.CreateCloudNodeRequest{
		Name:          nodeName,
		Endpoint:      "http://127.0.0.1:19000",
		Bucket:        "cubycloud-dev",
		Region:        "us-east-1",
		AccessKey:     "cubycloud-app",
		SecretKey:     "cubycloud-app-local-only-2026",
		CapacityCapGB: 100,
		Enabled:       true,
	})
	require.NoError(t, err)
	require.Equal(t, "S3", cloudNode.Type)

	// Verify credentials in SQLite are ENCRYPTED (not plaintext)
	var rawAccess, rawSecret []byte
	err = db.QueryRowContext(ctx, `SELECT access_key_encrypted, secret_key_encrypted FROM storage_nodes WHERE id = ?`, cloudNode.ID).Scan(&rawAccess, &rawSecret)
	require.NoError(t, err)
	require.NotEqual(t, "cubycloud-app", string(rawAccess), "Access key must be stored encrypted")
	require.NotEqual(t, "cubycloud-app-local-only-2026", string(rawSecret), "Secret key must be stored encrypted")

	// Set placement strategy to MOST_FREE_SPACE so cloud node is selected
	_, err = db.ExecContext(ctx, `INSERT OR REPLACE INTO settings (key, value) VALUES ('placement_strategy', 'MOST_FREE_SPACE')`)
	require.NoError(t, err)

	// 3. Upload File to S3
	testContent := []byte(fmt.Sprintf("Hello CubyCloud MinIO Storage! Timestamp: %d", time.Now().UnixNano()))
	filename := "test-s3-file.txt"

	intent, err := myStorageSvc.UploadIntent(ctx, userID, filename, int64(len(testContent)), nil, true)
	require.NoError(t, err)
	require.Equal(t, "s3", intent.Placement)
	require.NotEmpty(t, intent.UploadID)
	require.Contains(t, intent.URL, "127.0.0.1:19000/cubycloud-dev")

	// Direct PUT to MinIO URL
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, intent.URL, bytes.NewReader(testContent))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	httpResp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	httpResp.Body.Close()

	// Sync Cloud metadata to SQLite
	item, err := myStorageSvc.SyncCloud(ctx, userID, intent.UploadID)
	require.NoError(t, err)
	require.Equal(t, filename, item.Name)
	require.Equal(t, int64(len(testContent)), item.SizeBytes)

	// 4. SIMULATE FULL BACKEND RESTART
	// Close current DB connection and release driver
	require.NoError(t, db.Close())

	// Re-open DB and instantiate brand new StorageDriver and Services
	dbRestarted, err := database.Connect(tempDir)
	require.NoError(t, err)
	defer dbRestarted.Close()

	driverRestarted := storage.NewStorageDriver(dbRestarted, storagePath)
	myStorageSvcRestarted := mystorage.NewService(dbRestarted, driverRestarted)

	// 5. Verify File Accessible and Content Identical after Restart
	downloadedItem, r, err := myStorageSvcRestarted.Download(ctx, userID, item.ID)
	require.NoError(t, err)
	defer r.Close()

	downloadedBytes, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, testContent, downloadedBytes, "Content after backend restart must match uploaded content byte-for-byte")
	require.Equal(t, item.Name, downloadedItem.Name)

	// Also verify Presigned Download URL after restart
	dlResult, err := myStorageSvcRestarted.GetDownloadURL(ctx, userID, item.ID)
	require.NoError(t, err)
	require.NotNil(t, dlResult.URL)
	require.Contains(t, *dlResult.URL, "127.0.0.1:19000/cubycloud-dev")

	// Download via Presigned URL and verify identical bytes
	reqDL, err := http.NewRequestWithContext(ctx, http.MethodGet, *dlResult.URL, nil)
	require.NoError(t, err)
	respDL, err := client.Do(reqDL)
	require.NoError(t, err)
	defer respDL.Body.Close()
	require.Equal(t, http.StatusOK, respDL.StatusCode)
	bodyDL, err := io.ReadAll(respDL.Body)
	require.NoError(t, err)
	require.Equal(t, testContent, bodyDL)

	// 6. Delete File (Soft-delete to trash, then PurgeItem)
	require.NoError(t, myStorageSvcRestarted.Delete(ctx, userID, item.ID))
	require.NoError(t, myStorageSvcRestarted.PurgeItem(ctx, userID, item.ID))

	// Verify file record is gone from DB
	_, _, err = myStorageSvcRestarted.Download(ctx, userID, item.ID)
	require.Error(t, err)

	// Verify object physically deleted from MinIO
	cloudDriver, ok := driverRestarted.(storage.CloudStorageDriver)
	require.True(t, ok)
	_, headErr := cloudDriver.HeadObject(ctx, cloudNode.ID, item.ID)
	require.Error(t, headErr, "Object should be deleted from MinIO after PurgeItem")
}
