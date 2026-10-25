package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

func TestCORS_Origins(t *testing.T) {
	t.Run("wildcard in development", func(t *testing.T) {
		e := echo.New()
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		}))
		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://localhost:5188")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("explicit origins in production", func(t *testing.T) {
		e := echo.New()
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: []string{"https://app.example.com", "https://drive.example.com"},
			AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		}))
		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		// Allowed origin
		reqAllowed := httptest.NewRequest(http.MethodGet, "/test", nil)
		reqAllowed.Header.Set("Origin", "https://app.example.com")
		recAllowed := httptest.NewRecorder()
		e.ServeHTTP(recAllowed, reqAllowed)

		assert.Equal(t, http.StatusOK, recAllowed.Code)
		assert.Equal(t, "https://app.example.com", recAllowed.Header().Get("Access-Control-Allow-Origin"))

		// Disallowed origin
		reqDisallowed := httptest.NewRequest(http.MethodGet, "/test", nil)
		reqDisallowed.Header.Set("Origin", "https://evil.com")
		recDisallowed := httptest.NewRecorder()
		e.ServeHTTP(recDisallowed, reqDisallowed)

		assert.Equal(t, http.StatusOK, recDisallowed.Code)
		assert.Empty(t, recDisallowed.Header().Get("Access-Control-Allow-Origin"))
	})
}
