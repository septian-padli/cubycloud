package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/septian-padli/cubycloud/apps/backend/internal/config"
)

func main() {
	cfg := config.Load()
	if err := cfg.EnsureDataDirs(); err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.HideBanner = true
	e.GET("/health", health)
	e.GET("/api/v1/health", health)

	log.Printf("listening on %s (data=%s)", cfg.Addr(), cfg.DataDirPath)
	if err := e.Start(cfg.Addr()); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
