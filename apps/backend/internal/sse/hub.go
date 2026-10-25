package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
)

// Event represents a Server-Sent Event to be sent to clients.
type Event struct {
	Type   string `json:"type"`
	UserID string `json:"user_id,omitempty"`
	Data   any    `json:"data"`
}

// Hub manages connected SSE clients and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]string // channel -> userID
}

// NewHub initializes an SSE Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan Event]string),
	}
}

// Broadcast sends an event to all matching clients.
// If event.UserID is non-empty, only clients for that userID receive the event.
func (h *Hub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch, uid := range h.clients {
		if event.UserID == "" || event.UserID == uid {
			select {
			case ch <- event:
			default:
				// Skip if client buffer is full to prevent blocking
			}
		}
	}
}

// Handler returns an Echo handler streaming Server-Sent Events.
func (h *Hub) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, _ := c.Get("user_id").(string)

		c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
		c.Response().Header().Set("X-Accel-Buffering", "no")
		c.Response().WriteHeader(http.StatusOK)

		ch := make(chan Event, 32)
		h.mu.Lock()
		h.clients[ch] = userID
		h.mu.Unlock()

		defer func() {
			h.mu.Lock()
			delete(h.clients, ch)
			close(ch)
			h.mu.Unlock()
		}()

		// Send initial ping event
		_, _ = fmt.Fprint(c.Response(), "event: connected\ndata: {}\n\n")
		c.Response().Flush()

		ctx := c.Request().Context()
		for {
			select {
			case <-ctx.Done():
				return nil
			case ev, ok := <-ch:
				if !ok {
					return nil
				}
				payload, err := json.Marshal(ev.Data)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(c.Response(), "event: %s\ndata: %s\n\n", ev.Type, string(payload)); err != nil {
					return err
				}
				c.Response().Flush()
			}
		}
	}
}
