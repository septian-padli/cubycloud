package main

import (
	"context"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/cubycloud/backend/internal/config"
	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/jobs"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/admin"
	"github.com/cubycloud/backend/internal/modules/auth"
	"github.com/cubycloud/backend/internal/modules/docs"
	jobsmod "github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/modules/search"
	"github.com/cubycloud/backend/internal/modules/setup"
	"github.com/cubycloud/backend/internal/modules/share"
	"github.com/cubycloud/backend/internal/sse"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/cubycloud/backend/web"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/uptrace/bun"
)

func main() {
	// 1. Load config
	cfg := config.Load()
	log.Printf("Starting CubyCloud on port %s", cfg.Port)

	// 2. Ensure directories exist
	for _, dir := range []string{cfg.DBPath, cfg.StoragePath, cfg.TempTUSPath} {
		dirToCreate := dir
		if dir == cfg.DBPath {
			dirToCreate = cfg.DataDirPath + "/db"
		}
		if err := os.MkdirAll(dirToCreate, 0755); err != nil {
			log.Fatal("Failed to create directory:", err)
		}
	}

	// 3. Connect DB + run migrations
	db, err := database.Connect(cfg.DataDirPath)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer db.Close()

	// 4. Initialize storage master key
	if _, err := storage.InitMasterKey(cfg.DataDirPath, cfg.MasterKey, cfg.MasterKeyFile, db); err != nil {
		log.Fatal("Master key initialization failed:", err)
	}

	// 5. Init storage driver
	driver := storage.NewStorageDriver(db, cfg.StoragePath)

	// Reconcile orphan blobs across local disk and S3 nodes on startup (dryRun by default)
	if orphanReport, err := storage.ReconcileOrphanBlobs(context.Background(), db, driver, 72*time.Hour, cfg.StorageCleanupDryRun); err != nil {
		log.Printf("[STARTUP_ORPHAN_AUDIT] Error reconciling orphan blobs: %v", err)
	} else if orphanReport.LocalDeleted > 0 || orphanReport.S3Deleted > 0 || orphanReport.LocalScanned > 0 || orphanReport.S3Scanned > 0 {
		log.Printf("[STARTUP_ORPHAN_AUDIT] Reconciled orphans (dryRun=%v): local (scanned=%d, deleted=%d), S3 (scanned=%d, deleted=%d)",
			cfg.StorageCleanupDryRun, orphanReport.LocalScanned, orphanReport.LocalDeleted, orphanReport.S3Scanned, orphanReport.S3Deleted)
	}

	// 5. Init setup guard cache
	setupGuard := initSetupGuard(db)

	// 6. Init SSE Hub & Job Queue
	sseHub := sse.NewHub()
	jobQueue := jobsmod.NewQueue(db)

	// 7. Worker pool & handlers
	workerPool := jobsmod.NewWorkerPool(db, jobQueue, sseHub)
	compressHandler := jobsmod.NewCompressHandler(db, driver)
	workerPool.RegisterHandler(jobsmod.TypeCompressZip, compressHandler.Execute)
	nodeMigrationHandler := jobsmod.NewNodeMigrationHandler(db, driver)
	workerPool.RegisterHandler(jobsmod.TypeNodeMigration, nodeMigrationHandler.Execute)
	workerPool.Start(context.Background())
	defer workerPool.Stop()

	// 8. Build router
	e := buildEchoRouter(cfg, db, driver, setupGuard, sseHub, jobQueue, workerPool)
	// Background cron worker for 30-day trash auto-purge
	trashPurgeJob := jobs.StartTrashPurgeJob(context.Background(), db, driver, 1*time.Hour, 30*24*time.Hour)
	defer trashPurgeJob.Stop()
	auditCleanupJob := jobs.StartAuditCleanupJob(context.Background(), db, time.Hour)
	defer auditCleanupJob.Stop()

	// 9. Serve embedded frontend
	distFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal("Embed FS failed:", err)
	}
	e.GET("/*", echo.WrapHandler(newSPAHandler(distFS)))

	// 10. Start server
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server start failed:", err)
		}
	}()

	log.Printf("Server ready at http://localhost:%s", cfg.Port)

	// 11. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatal("Shutdown failed:", err)
	}
	log.Println("Server stopped")
}
func newSPAHandler(distFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(distFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath != "" && cleanPath != "." {
			if f, err := distFS.Open(cleanPath); err == nil {
				defer f.Close()
				if stat, err := f.Stat(); err == nil && !stat.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}

		indexFile, err := distFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer indexFile.Close()

		stat, err := indexFile.Stat()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if seeker, ok := indexFile.(io.ReadSeeker); ok {
			http.ServeContent(w, r, "index.html", stat.ModTime(), seeker)
			return
		}

		data, err := io.ReadAll(indexFile)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func buildEchoRouter(cfg *config.Config, db *bun.DB, driver storage.StorageDriver, setupGuard *middleware.SetupGuard, sseHub *sse.Hub, jobQueue *jobsmod.Queue, workerPool ...*jobsmod.WorkerPool) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	if len(cfg.AllowedOrigins) > 0 {
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: cfg.AllowedOrigins,
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		}))
	}
	e.Use(middleware.AuditLog(db))
	healthHandler := func(c echo.Context) error {
		if err := db.PingContext(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}
		if !storage.AllConfiguredNodesAvailable(c.Request().Context(), db, driver) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
	e.GET("/health", healthHandler)
	e.GET("/api/health", healthHandler)

	// Routes
	setup.RegisterRoutes(e, db, driver, setupGuard, cfg.DataDirPath, cfg.JWTSecret)

	// Admin routes (JWT auth + Super Admin guard, bypasses setup guard)
	dbPath := cfg.DataDirPath + "/db/cubycloud.db"
	jwtAuth := middleware.JWTAuth(cfg.JWTSecret, db)
	adminGuard := middleware.SuperAdminGuard(db)

	// Real-time SSE hub
	e.GET("/api/v1/events", sseHub.Handler(), jwtAuth)

	// SQLite transactional job queue & bounded worker pool
	jobsmod.RegisterRoutes(e, jobQueue, jwtAuth)

	adminHandler := admin.RegisterRoutes(e, db, driver, cfg.DataDirPath, dbPath, jwtAuth, adminGuard, jobQueue)
	if len(workerPool) > 0 && workerPool[0] != nil {
		adminHandler.SetConcurrencyController(workerPool[0])
	}
	docs.RegisterRoutes(e, jwtAuth, adminGuard)
	// Auth routes (login, logout, me, reset-password)
	auth.RegisterRoutes(e, db, cfg.JWTSecret, jwtAuth)

	// My Storage VFS endpoints (authenticated per-user file operations & trash).
	myStorageHandler := mystorage.RegisterRoutes(e, db, driver, jwtAuth, jobQueue)
	driveGroup := e.Group("/api/v1/drive", jwtAuth)
	driveGroup.PATCH("/items/:id/move", myStorageHandler.MoveItem)
	driveGroup.POST("/items/:id/star", myStorageHandler.ToggleStar)
	driveGroup.PUT("/items/:id/star", myStorageHandler.ToggleStar)
	driveGroup.GET("/starred", myStorageHandler.ListStarred)
	driveGroup.POST("/sync-cloud", myStorageHandler.SyncCloud)
	driveGroup.GET("/home", myStorageHandler.GetHome)
	driveGroup.POST("/access", myStorageHandler.RecordAccess)
	organization.RegisterRoutes(e, db, driver, jwtAuth, jobQueue)
	// Share endpoints (internal user grants & public share links)
	share.RegisterRoutes(e, db, driver, jwtAuth)
	// Global search endpoint across My Storage, Organization, and Shared With Me
	search.RegisterRoutes(e, db, jwtAuth)

	return e
}

func initSetupGuard(db *bun.DB) *middleware.SetupGuard {
	var isCompleted string
	err := db.NewSelect().Table("settings").Column("value").Where("key = ?", "is_setup_completed").Scan(context.Background(), &isCompleted)
	return middleware.NewSetupGuard(err == nil && isCompleted == "true")
}
