package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/admin"
	"github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestDetachStorageNodeLifecycle(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))

	db, err := database.Connect(tempDir)
	require.NoError(t, err)
	defer db.Close()

	storagePath := filepath.Join(tempDir, "storage")
	require.NoError(t, os.MkdirAll(storagePath, 0755))
	driver := storage.NewStorageDriver(db, storagePath)

	queue := jobs.NewQueue(db)
	service := admin.NewService(db, driver, tempDir, queue)

	node1Path := filepath.Join(storagePath, "n1")
	node2Path := filepath.Join(storagePath, "n2")
	require.NoError(t, os.MkdirAll(node1Path, 0755))
	require.NoError(t, os.MkdirAll(node2Path, 0755))

	node1 := &database.StorageNode{
		ID:            "test-node-1",
		Name:          "Node 1",
		Type:          "LOCAL",
		Path:          &node1Path,
		CapacityBytes: 10 * 1024 * 1024 * 1024,
		Enabled:       true,
		CreatedAt:     time.Now(),
	}
	node2 := &database.StorageNode{
		ID:            "test-node-2",
		Name:          "Node 2",
		Type:          "LOCAL",
		Path:          &node2Path,
		CapacityBytes: 10 * 1024 * 1024 * 1024,
		Enabled:       true,
		CreatedAt:     time.Now(),
	}
	_, err = db.NewInsert().Model(node1).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewInsert().Model(node2).Exec(ctx)
	require.NoError(t, err)

	// Insert user
	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, role)
		VALUES ('admin-user', 'admin', 'admin@example.test', 'hash', 'SUPER_ADMIN')`)
	require.NoError(t, err)

	// 1. Detach an empty node -> immediately safe to disconnect, no job needed
	respEmpty, err := service.DetachStorageNode(ctx, node2.ID, "", "admin-user")
	require.NoError(t, err)
	require.True(t, respEmpty.SafeToDisconnect)
	require.Nil(t, respEmpty.JobID)

	nodes, err := service.GetStorageNodes(ctx)
	require.NoError(t, err)
	var n2Resp *admin.StorageNodeResponse
	for i := range nodes {
		if nodes[i].ID == node2.ID {
			n2Resp = &nodes[i]
		}
	}
	require.NotNil(t, n2Resp)
	require.False(t, n2Resp.Enabled)
	require.True(t, n2Resp.SafeToDisconnect)

	// 2. Detach a node with files -> not safe to disconnect yet, background migration job enqueued
	itemID := ulid.Make().String()
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, storage_node_id, modified_at)
		VALUES (?, 'admin-user', 'data.bin', 'file', 1024, 'key-1024', ?, CURRENT_TIMESTAMP)`, itemID, node1.ID)
	require.NoError(t, err)

	respWithFiles, err := service.DetachStorageNode(ctx, node1.ID, node2.ID, "admin-user")
	require.NoError(t, err)
	require.False(t, respWithFiles.SafeToDisconnect)
	require.NotNil(t, respWithFiles.JobID)

	nodes, err = service.GetStorageNodes(ctx)
	require.NoError(t, err)
	var n1Resp *admin.StorageNodeResponse
	for i := range nodes {
		if nodes[i].ID == node1.ID {
			n1Resp = &nodes[i]
		}
	}
	require.NotNil(t, n1Resp)
	require.False(t, n1Resp.Enabled)
	require.False(t, n1Resp.SafeToDisconnect, "node with files must not be safe to disconnect")
	require.Equal(t, "MIGRATING", n1Resp.MigrationStatus)
	require.Equal(t, *respWithFiles.JobID, n1Resp.MigrationJobID)

	// Test HTTP endpoint via Echo handler
	e := echo.New()
	handler := admin.NewHandler(db, driver, tempDir, filepath.Join(tempDir, "db", "cubycloud.db"), queue)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/storage-nodes/"+node1.ID+"/detach", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(node1.ID)
	c.Set("user_id", "admin-user")

	err = handler.DetachStorageNode(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
}
