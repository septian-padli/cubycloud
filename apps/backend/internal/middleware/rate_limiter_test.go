package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}

	limit := 3
	window := 200 * time.Millisecond
	limiter := RateLimiter(limit, window)

	// Requests from IP 1.2.3.4
	for range limit {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set("X-Real-IP", "1.2.3.4")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := limiter(handler)(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// Next request from same IP should be blocked (429)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Real-IP", "1.2.3.4")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := limiter(handler)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "too many requests")

	// Request from different IP 5.6.7.8 should succeed
	reqOther := httptest.NewRequest(http.MethodPost, "/test", nil)
	reqOther.Header.Set("X-Real-IP", "5.6.7.8")
	recOther := httptest.NewRecorder()
	cOther := e.NewContext(reqOther, recOther)

	errOther := limiter(handler)(cOther)
	assert.NoError(t, errOther)
	assert.Equal(t, http.StatusOK, recOther.Code)

	// Wait for window to expire
	time.Sleep(window + 50*time.Millisecond)

	// Original IP should now succeed again
	reqRetry := httptest.NewRequest(http.MethodPost, "/test", nil)
	reqRetry.Header.Set("X-Real-IP", "1.2.3.4")
	recRetry := httptest.NewRecorder()
	cRetry := e.NewContext(reqRetry, recRetry)

	errRetry := limiter(handler)(cRetry)
	assert.NoError(t, errRetry)
	assert.Equal(t, http.StatusOK, recRetry.Code)
}

func TestRateLimiterRestoresSnapshotAfterRestart(t *testing.T) {
	t.Setenv("DISABLE_RATE_LIMIT", "")
	t.Setenv("MY_STORAGE_E2E_DATA_DIR", "")
	statePath := filepath.Join(t.TempDir(), "rate_limiter_state.json")
	e := echo.New()
	handler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	request := func(limiter echo.MiddlewareFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Real-IP", "192.0.2.1")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		assert.NoError(t, limiter(handler)(c))
		return rec
	}
	first := RateLimiterWithFile(1, time.Minute, statePath)
	assert.Equal(t, http.StatusOK, request(first).Code)
	second := RateLimiterWithFile(1, time.Minute, statePath)
	assert.Equal(t, http.StatusTooManyRequests, request(second).Code)
}

func TestRateLimiterScopedIsolationAndPersistence(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR_PATH", dataDir)
	t.Setenv("RATE_LIMITER_STATE_PATH", "")
	t.Setenv("DISABLE_RATE_LIMIT", "")
	t.Setenv("MY_STORAGE_E2E_DATA_DIR", "")

	e := echo.New()
	handler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	req := func(limiter echo.MiddlewareFunc, ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("X-Real-IP", ip)
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		assert.NoError(t, limiter(handler)(c))
		return w
	}

	loginLimiter := RateLimiterScoped("auth_login", 1, time.Minute)
	shareLimiter := RateLimiterScoped("share_unlock", 1, time.Minute)

	// Exhaust login limiter
	assert.Equal(t, http.StatusOK, req(loginLimiter, "10.0.0.1").Code)
	assert.Equal(t, http.StatusTooManyRequests, req(loginLimiter, "10.0.0.1").Code)

	// Share limiter for same IP must still be independent and succeed
	assert.Equal(t, http.StatusOK, req(shareLimiter, "10.0.0.1").Code)
	assert.Equal(t, http.StatusTooManyRequests, req(shareLimiter, "10.0.0.1").Code)

	// Restart: recreate limiters from the persisted scopes
	restartedLogin := RateLimiterScoped("auth_login", 1, time.Minute)
	restartedShare := RateLimiterScoped("share_unlock", 1, time.Minute)
	assert.Equal(t, http.StatusTooManyRequests, req(restartedLogin, "10.0.0.1").Code)
	assert.Equal(t, http.StatusTooManyRequests, req(restartedShare, "10.0.0.1").Code)
}

func TestRateLimiterFailClosedOnCorruptOrUnwritableState(t *testing.T) {
	t.Setenv("DISABLE_RATE_LIMIT", "")
	t.Setenv("MY_STORAGE_E2E_DATA_DIR", "")
	e := echo.New()
	handler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	req := func(limiter echo.MiddlewareFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("X-Real-IP", "127.0.0.1")
		w := httptest.NewRecorder()
		c := e.NewContext(r, w)
		assert.NoError(t, limiter(handler)(c))
		return w
	}

	// 1. Corrupt state file -> 503
	corruptPath := filepath.Join(t.TempDir(), "corrupt.json")
	assert.NoError(t, os.WriteFile(corruptPath, []byte("invalid-json"), 0600))
	corruptLimiter := RateLimiterWithFile(5, time.Minute, corruptPath)
	assert.Equal(t, http.StatusServiceUnavailable, req(corruptLimiter).Code)

	// 2. Unwritable state path directory -> 503 on write
	filePath := filepath.Join(t.TempDir(), "blocker-file")
	assert.NoError(t, os.WriteFile(filePath, []byte("x"), 0600))
	unwritablePath := filepath.Join(filePath, "child", "limiter.json")
	unwritableLimiter := RateLimiterWithFile(5, time.Minute, unwritablePath)
	assert.Equal(t, http.StatusServiceUnavailable, req(unwritableLimiter).Code)
}
