package httputil

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestIsSecureRequestAndCookie(t *testing.T) {
	// Ensure clean env after tests
	originalEnv := os.Getenv("COOKIE_SECURE")
	defer func() {
		_ = os.Setenv("COOKIE_SECURE", originalEnv)
	}()

	t.Run("default plain http without env or headers", func(t *testing.T) {
		_ = os.Unsetenv("COOKIE_SECURE")
		req := httptest.NewRequest(http.MethodGet, "http://localhost:8383/api/v1/auth/login", nil)
		if IsSecureRequest(req) {
			t.Errorf("expected plain HTTP request to be not secure")
		}

		e := echo.New()
		c := e.NewContext(req, httptest.NewRecorder())
		if IsSecureCookie(c) {
			t.Errorf("expected plain HTTP context cookie to be not secure")
		}
	})

	t.Run("COOKIE_SECURE=true forces secure regardless of connection", func(t *testing.T) {
		_ = os.Setenv("COOKIE_SECURE", "true")
		req := httptest.NewRequest(http.MethodGet, "http://localhost:8383/api/v1/auth/login", nil)
		if !IsSecureRequest(req) {
			t.Errorf("expected IsSecureRequest to be true when COOKIE_SECURE=true")
		}

		e := echo.New()
		c := e.NewContext(req, httptest.NewRecorder())
		if !IsSecureCookie(c) {
			t.Errorf("expected IsSecureCookie to be true when COOKIE_SECURE=true")
		}
	})

	t.Run("direct TLS connection", func(t *testing.T) {
		_ = os.Unsetenv("COOKIE_SECURE")
		req := httptest.NewRequest(http.MethodGet, "https://localhost:8383/api/v1/auth/login", nil)
		req.TLS = &tls.ConnectionState{}
		if !IsSecureRequest(req) {
			t.Errorf("expected TLS request to be secure")
		}

		e := echo.New()
		c := e.NewContext(req, httptest.NewRecorder())
		if !IsSecureCookie(c) {
			t.Errorf("expected TLS context cookie to be secure")
		}
	})

	t.Run("reverse proxy with X-Forwarded-Proto https", func(t *testing.T) {
		_ = os.Unsetenv("COOKIE_SECURE")
		req := httptest.NewRequest(http.MethodGet, "http://localhost:8383/api/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		if !IsSecureRequest(req) {
			t.Errorf("expected X-Forwarded-Proto https request to be secure")
		}

		e := echo.New()
		c := e.NewContext(req, httptest.NewRecorder())
		if !IsSecureCookie(c) {
			t.Errorf("expected X-Forwarded-Proto https context cookie to be secure")
		}
	})

	t.Run("reverse proxy chain with multiple protocols in X-Forwarded-Proto", func(t *testing.T) {
		_ = os.Unsetenv("COOKIE_SECURE")
		req := httptest.NewRequest(http.MethodGet, "http://localhost:8383/api/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-Proto", "https, http")
		if !IsSecureRequest(req) {
			t.Errorf("expected chained X-Forwarded-Proto starting with https to be secure")
		}

		reqInsecure := httptest.NewRequest(http.MethodGet, "http://localhost:8383/api/v1/auth/login", nil)
		reqInsecure.Header.Set("X-Forwarded-Proto", "http, https")
		if IsSecureRequest(reqInsecure) {
			t.Errorf("expected chained X-Forwarded-Proto starting with http to be not secure")
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		_ = os.Unsetenv("COOKIE_SECURE")
		if IsSecureRequest(nil) {
			t.Errorf("nil request should return false")
		}
		if IsSecureCookie(nil) {
			t.Errorf("nil context should return false")
		}
	})
}
