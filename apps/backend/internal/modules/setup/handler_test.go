package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanNodes(t *testing.T) {
	// Setup temp data dir
	tmpDir := t.TempDir()
	storageRoot := filepath.Join(tmpDir, "storage")
	require.NoError(t, os.MkdirAll(storageRoot, 0755))

	// Create test nodes
	node1 := filepath.Join(storageRoot, "node1")
	node2 := filepath.Join(storageRoot, "node2")
	require.NoError(t, os.MkdirAll(node1, 0755))
	require.NoError(t, os.MkdirAll(node2, 0755))

	// Init handler
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)
	setupGuard := middleware.NewSetupGuard(false)
	storageDriver := storage.NewLocalDriver(storageRoot)
	handler := NewHandler(db, storageDriver, setupGuard, tmpDir, "test-secret")

	// Make request
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/scan-nodes", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ScanNodes(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Parse response
	var resp ScanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// Verify
	assert.Len(t, resp.Nodes, 2)
	assert.Empty(t, resp.Devices)
	
	for _, node := range resp.Nodes {
		assert.Contains(t, []string{node1, node2}, node.Path)
		assert.Greater(t, node.CapacityBytes, int64(0))
		assert.Greater(t, node.FreeBytes, int64(0))
	}
}

func TestScanNodes_EmptyStorage(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create storage dir
	
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)
	setupGuard := middleware.NewSetupGuard(false)
	storageDriver := storage.NewLocalDriver(filepath.Join(tmpDir, "storage"))
	handler := NewHandler(db, storageDriver, setupGuard, tmpDir, "test-secret")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/scan-nodes", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ScanNodes(c)
	require.NoError(t, err)

	var resp ScanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Nodes, 1)
	assert.Equal(t, filepath.Join(tmpDir, "storage", "default"), resp.Nodes[0].Path)
	assert.Empty(t, resp.Devices)
}
