package admin

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/stretchr/testify/require"
)

type mockCloudStorageDriver struct {
	mu                  sync.Mutex
	deletedCloudObjects []string
	deletedLocalFiles   []string
}

func (m *mockCloudStorageDriver) WriteLocal(_ context.Context, _, _ string, _ io.Reader) error {
	return nil
}

func (m *mockCloudStorageDriver) ReadLocal(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockCloudStorageDriver) DeleteLocal(_ context.Context, nodePath, filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedLocalFiles = append(m.deletedLocalFiles, fmt.Sprintf("%s/%s", nodePath, filename))
	return nil
}

func (m *mockCloudStorageDriver) StatLocal(context.Context, string) (int64, int64, error) {
	return 10000000000, 5000000000, nil
}

func (m *mockCloudStorageDriver) GeneratePresignedUploadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (m *mockCloudStorageDriver) GeneratePresignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (m *mockCloudStorageDriver) ValidateCloudCredentials(context.Context, string, string, string, string, string) error {
	return nil
}

func (m *mockCloudStorageDriver) ReadObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockCloudStorageDriver) WriteObject(context.Context, string, string, io.Reader, int64) error {
	return nil
}

func (m *mockCloudStorageDriver) DeleteObject(_ context.Context, nodeID, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedCloudObjects = append(m.deletedCloudObjects, fmt.Sprintf("%s:%s", nodeID, key))
	return nil
}

func (m *mockCloudStorageDriver) HeadObject(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (m *mockCloudStorageDriver) ListObjects(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (m *mockCloudStorageDriver) PresignUpload(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (m *mockCloudStorageDriver) PresignDownload(context.Context, string, string, string, time.Duration) (string, error) {
	return "", nil
}

func TestDeleteUser_S3BlobCleanupAndOrganizationPreservation(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	dbDir := filepath.Join(dataDir, "db")
	require.NoError(t, os.MkdirAll(dbDir, 0o755))

	db, err := database.Connect(dataDir)
	require.NoError(t, err)
	defer db.Close()

	driver := &mockCloudStorageDriver{}
	adminSvc := NewService(db, driver, dataDir)
	orgSvc := organization.NewService(db, driver)

	// 1. Setup Storage Nodes (S3 and Local)
	_, err = db.ExecContext(ctx, `
		INSERT INTO storage_nodes (id, name, type, capacity_bytes, enabled)
		VALUES ('s3-node-1', 'S3 Cloud Node', 'S3', 10000000, 1)
	`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO storage_nodes (id, name, type, path, capacity_bytes, enabled)
		VALUES ('local-node-1', 'Local Disk Node', 'LOCAL', '/data/local', 10000000, 1)
	`)
	require.NoError(t, err)

	// 2. Setup Users: target user (to be deleted) and alice (other user)
	targetID := "user-target"
	aliceID := "user-alice"
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, username, email, password_hash, role)
		VALUES
			(?, 'targetuser', 'target@example.com', 'hash', 'REGULAR'),
			(?, 'alice', 'alice@example.com', 'hash', 'REGULAR')
	`, targetID, aliceID)
	require.NoError(t, err)

	// 3. Setup target user's personal items:
	// - S3 file (exclusive ref)
	// - Local file (exclusive ref)
	// - S3 file in trash (exclusive ref)
	// - S3 file deduplicated with an organization item (shared ref)
	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id)
		VALUES
			('pers-s3-1', ?, 'cloud.pdf', 'file', 1024, 'blob-s3-pers', 's3-node-1'),
			('pers-local-1', ?, 'local.pdf', 'file', 2048, 'blob-local-pers', 'local-node-1'),
			('pers-shared-1', ?, 'shared.pdf', 'file', 512, 'blob-shared', 's3-node-1')
	`, targetID, targetID, targetID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id, deleted_at)
		VALUES
			('pers-s3-trash', ?, 'trash.pdf', 'file', 4096, 'blob-s3-trash', 's3-node-1', CURRENT_TIMESTAMP)
	`, targetID)
	require.NoError(t, err)

	// 4. Setup organization items:
	// - S3 file uploaded by target user
	// - Local file uploaded by target user
	// - S3 file referencing the shared disk key
	// - Folder created by target user
	_, err = db.ExecContext(ctx, `
		INSERT INTO organization_items (id, created_by, name, kind, size_bytes, disk_key, storage_node_id)
		VALUES
			('org-s3-1', ?, 'org-cloud.pdf', 'file', 1024, 'blob-s3-org', 's3-node-1'),
			('org-local-1', ?, 'org-local.pdf', 'file', 2048, 'blob-local-org', 'local-node-1'),
			('org-shared-1', ?, 'org-shared.pdf', 'file', 512, 'blob-shared', 's3-node-1'),
			('org-folder-1', ?, 'Org Team Folder', 'folder', 0, NULL, NULL)
	`, targetID, targetID, aliceID, targetID)
	require.NoError(t, err)

	// 5. Delete User via Admin Service
	err = adminSvc.DeleteUser(ctx, "actor-admin", targetID, &DeleteUserRequest{
		Email: "target@example.com",
	})
	require.NoError(t, err)

	// 6. Verify physical deletion:
	// - Personal S3 blobs with 0 ref count MUST be deleted via DeleteObject
	require.Contains(t, driver.deletedCloudObjects, "s3-node-1:blob-s3-pers")
	require.Contains(t, driver.deletedCloudObjects, "s3-node-1:blob-s3-trash")

	// - Personal Local blob with 0 ref count MUST be deleted via DeleteLocal
	require.Contains(t, driver.deletedLocalFiles, "/data/local/blob-local-pers")

	// - Shared blob MUST NOT be physically deleted because org-shared-1 still references it
	require.NotContains(t, driver.deletedCloudObjects, "s3-node-1:blob-shared")

	// - Org files MUST NOT be physically deleted
	require.NotContains(t, driver.deletedCloudObjects, "s3-node-1:blob-s3-org")
	require.NotContains(t, driver.deletedLocalFiles, "/data/local/blob-local-org")

	// 7. Verify database state for personal items:
	// - All my_storage_items for target user must be deleted
	var persCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM my_storage_items WHERE owner_id = ?`, targetID).Scan(&persCount)
	require.NoError(t, err)
	require.Equal(t, 0, persCount, "all personal items for target user must be deleted")

	// 8. Verify database state for organization items:
	// - All organization items must still exist
	var orgCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_items`).Scan(&orgCount)
	require.NoError(t, err)
	require.Equal(t, 4, orgCount, "organization items must remain intact")

	// 9. Verify user tombstone in users table
	var tombstone struct {
		Username  string
		Email     string
		DeletedAt *string
	}
	err = db.QueryRowContext(ctx, `SELECT username, email, deleted_at FROM users WHERE id = ?`, targetID).
		Scan(&tombstone.Username, &tombstone.Email, &tombstone.DeletedAt)
	require.NoError(t, err)
	require.NotNil(t, tombstone.DeletedAt, "deleted_at must be populated")
	require.Equal(t, fmt.Sprintf("deleted_user_%s", targetID), tombstone.Username)
	require.Equal(t, fmt.Sprintf("deleted_%s@cubycloud.local", targetID), tombstone.Email)

	// 10. Verify Organization Service List:
	// Items created by target user must display created_by as 'deleted user'
	items, _, err := orgSvc.List(ctx, aliceID, nil)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	for _, item := range items {
		switch item.ID {
		case "org-s3-1", "org-local-1", "org-folder-1":
			require.Equal(t, "deleted user", item.CreatedBy, "item %s created_by should be 'deleted user'", item.ID)
		case "org-shared-1":
			require.Equal(t, "alice", item.CreatedBy, "item %s created_by should be 'alice'", item.ID)
		}
	}
}
