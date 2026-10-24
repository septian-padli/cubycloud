package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"
)

func TestSPAFallback(t *testing.T) {
	const index = "<!doctype html><html><body>app</body></html>"
	distFS := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte(index)},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}
	e := echo.New()
	e.GET("/*", echo.WrapHandler(newSPAHandler(fs.FS(distFS))))

	for _, path := range []string{"/login", "/drive/my-storage", "/"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if response.Body.String() != index {
				t.Fatalf("body = %q, want index.html %q", response.Body.String(), index)
			}
		})
	}

	t.Run("API not found", func(t *testing.T) {
		response := httptest.NewRecorder()
		e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
		}
		if strings.Contains(response.Body.String(), index) {
			t.Fatal("API response unexpectedly contains index.html")
		}
	})
}
