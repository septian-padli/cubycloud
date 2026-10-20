package middleware

import (
	"net/http"
	"net/http/httptest"
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
