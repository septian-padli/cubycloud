package main

import (
	"context"
	"github.com/cubycloud/backend/internal/modules/admin"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cubycloud/backend/internal/config"
	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/modules/auth"
	"github.com/cubycloud/backend/internal/modules/setup"
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
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// 8. Routes
	setup.RegisterRoutes(e, db, driver, setupGuard, cfg.DataDirPath, cfg.JWTSecret)
	
	// Admin routes (JWT auth + Super Admin guard, bypasses setup guard)
	dbPath := cfg.DataDirPath + "/db/cubycloud.db"
	jwtAuth := middleware.JWTAuth(cfg.JWTSecret)
	adminGuard := middleware.SuperAdminGuard(db)
	admin.RegisterRoutes(e, db, driver, cfg.DataDirPath, dbPath, jwtAuth, adminGuard)
	
	// Auth routes (password reset)
	authService := auth.NewService(db)
	authHandler := auth.NewHandler(authService)
	e.POST("/api/v1/auth/reset-password", authHandler.ResetPassword)

	// 9. Serve embedded frontend
	distFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal("Embed FS failed:", err)
	}
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(distFS))))

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

func initSetupGuard(db *bun.DB) *middleware.SetupGuard {
	var isCompleted string
	err := db.NewSelect().Table("settings").Column("value").Where("key = ?", "is_setup_completed").Scan(context.Background(), &isCompleted)
	return middleware.NewSetupGuard(err == nil && isCompleted == "true")
}
