package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cubycloud/backend/internal/config"
	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	jobsmod "github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/sse"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/stretchr/testify/require"
)

type RouteInfo struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Name   string `json:"name"`
}

func TestPrintRouteInventory(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "storage"), 0755))

	db, err := database.Connect(tempDir)
	require.NoError(t, err)
	defer db.Close()

	driver := storage.NewStorageDriver(db, filepath.Join(tempDir, "storage"))
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

	e := buildEchoRouter(cfg, db, driver, setupGuard, sseHub, jobQueue)

	var routes []RouteInfo
	for _, r := range e.Routes() {
		if r.Method == "echo_route_not_found" {
			continue
		}
		routes = append(routes, RouteInfo{
			Method: r.Method,
			Path:   r.Path,
			Name:   r.Name,
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	data, err := json.MarshalIndent(routes, "", "  ")
	require.NoError(t, err)

	fmt.Printf("\n=== REGISTERED ROUTE COUNT: %d ===\n", len(routes))
	fmt.Println(string(data))
}
