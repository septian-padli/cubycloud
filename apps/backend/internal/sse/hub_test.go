package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHub_BroadcastAndStream(t *testing.T) {
	hub := NewHub()
	e := echo.New()

	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(reqCtx)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user-123")

	errCh := make(chan error, 1)
	go func() {
		errCh <- hub.Handler()(c)
	}()

	// Wait for client to connect
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(Event{
		Type:   "job_updated",
		UserID: "user-123",
		Data:   map[string]string{"status": "COMPLETED"},
	})

	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	assert.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "event: connected")
	assert.Contains(t, body, "event: job_updated")
	assert.Contains(t, body, `"status":"COMPLETED"`)
}
