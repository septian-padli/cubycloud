package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cubycloud/backend/internal/config"
	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	jobsmod "github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/sse"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/stretchr/testify/require"
)

type healthTestDriver struct {
	storage.StorageDriver
	unavailableNodeID string
}

func (d *healthTestDriver) ProbeNode(_ context.Context, nodeID string) error {
	if d.unavailableNodeID != "" && d.unavailableNodeID == nodeID {
		return errors.New("node down")
	}
	return nil
}

func TestHealthEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "storage"), 0755))

	db, err := database.Connect(tempDir)
	require.NoError(t, err)
	defer db.Close()

	// Seed test storage nodes
	_, err = db.ExecContext(context.Background(), `INSERT INTO storage_nodes(id, type, capacity_bytes, enabled) VALUES(?, ?, ?, ?)`, "node-1", "LOCAL", 1000, true)
	require.NoError(t, err)

	cfg := &config.Config{
		Port:           "8383",
		DataDirPath:    tempDir,
		DBPath:         filepath.Join(tempDir, "db", "cubycloud.db"),
		StoragePath:    filepath.Join(tempDir, "storage"),
		JWTSecret:      "test-secret-at-least-32-bytes-long-super-secret",
		AllowedOrigins: []string{"*"},
	}
	setupGuard := middleware.NewSetupGuard(true)
	sseHub := sse.NewHub()
	jobQueue := jobsmod.NewQueue(db)

	mockDriver := &healthTestDriver{}
	e := buildEchoRouter(cfg, db, mockDriver, setupGuard, sseHub, jobQueue)
	// 1. When healthy -> 200 OK for /health and /api/health
	for _, path := range []string{"/health", "/api/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "ok", body["status"])
	}

	// 2. When storage node is unavailable -> 503 Service Unavailable
	{
		mockDriver.unavailableNodeID = "node-1"
		for _, path := range []string{"/health", "/api/health"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, "unavailable", body["status"])
		}
		mockDriver.unavailableNodeID = ""
	}

	// 3. When database connection fails / closes -> 503 Service Unavailable
	{
		require.NoError(t, db.Close())
		for _, path := range []string{"/health", "/api/health"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, "unavailable", body["status"])
		}
	}
}
