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
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/modules/search"
	"github.com/cubycloud/backend/internal/modules/setup"
	"github.com/cubycloud/backend/internal/modules/share"
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

	// 4. Init storage driver
	driver := storage.NewStorageDriver(db, cfg.StoragePath)

	// Reconcile orphan blobs across local disk and S3 nodes on startup
	if orphanReport, err := storage.ReconcileOrphanBlobs(context.Background(), db, driver, 5*time.Minute); err != nil {
		log.Printf("[STARTUP_ORPHAN_AUDIT] Error reconciling orphan blobs: %v", err)
	} else if orphanReport.LocalDeleted > 0 || orphanReport.S3Deleted > 0 {
		log.Printf("[STARTUP_ORPHAN_AUDIT] Reconciled orphans: local (scanned=%d, deleted=%d), S3 (scanned=%d, deleted=%d)",
			orphanReport.LocalScanned, orphanReport.LocalDeleted, orphanReport.S3Scanned, orphanReport.S3Deleted)
	}

	// 5. Init setup guard cache
	setupGuard := initSetupGuard(db)

	// 6. Create Echo
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// 7. Middleware
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
	}))
	e.Use(middleware.AuditLog(db))
	e.GET("/health", func(c echo.Context) error {
		if storage.AllConfiguredNodesAvailable(c.Request().Context(), db, driver) {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		}
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
	})

	// 8. Routes
	setup.RegisterRoutes(e, db, driver, setupGuard, cfg.DataDirPath, cfg.JWTSecret)

	// Admin routes (JWT auth + Super Admin guard, bypasses setup guard)
	dbPath := cfg.DataDirPath + "/db/cubycloud.db"
	jwtAuth := middleware.JWTAuth(cfg.JWTSecret, db)
	adminGuard := middleware.SuperAdminGuard(db)
	admin.RegisterRoutes(e, db, driver, cfg.DataDirPath, dbPath, jwtAuth, adminGuard)

	// Auth routes (login, logout, me, reset-password)
	auth.RegisterRoutes(e, db, cfg.JWTSecret, jwtAuth)

	// My Storage VFS endpoints (authenticated per-user file operations & trash).
	myStorageHandler := mystorage.RegisterRoutes(e, db, driver, jwtAuth)
	driveGroup := e.Group("/api/v1/drive", jwtAuth)
	driveGroup.PATCH("/items/:id/move", myStorageHandler.MoveItem)
	driveGroup.POST("/items/:id/star", myStorageHandler.ToggleStar)
	driveGroup.PUT("/items/:id/star", myStorageHandler.ToggleStar)
	driveGroup.GET("/starred", myStorageHandler.ListStarred)
	driveGroup.POST("/sync-cloud", myStorageHandler.SyncCloud)
	driveGroup.GET("/home", myStorageHandler.GetHome)
	driveGroup.POST("/access", myStorageHandler.RecordAccess)
	organization.RegisterRoutes(e, db, driver, jwtAuth)
	// Share endpoints (internal user grants & public share links)
	share.RegisterRoutes(e, db, driver, jwtAuth)
	// Global search endpoint across My Storage, Organization, and Shared With Me
	search.RegisterRoutes(e, db, jwtAuth)
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

func initSetupGuard(db *bun.DB) *middleware.SetupGuard {
	var isCompleted string
	err := db.NewSelect().Table("settings").Column("value").Where("key = ?", "is_setup_completed").Scan(context.Background(), &isCompleted)
	return middleware.NewSetupGuard(err == nil && isCompleted == "true")
}
