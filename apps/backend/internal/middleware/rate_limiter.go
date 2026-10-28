package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type ipBucket struct {
	Timestamps []time.Time `json:"timestamps"`
}

// RateLimiter returns an Echo middleware that limits requests by client IP with default global state.
func RateLimiter(limit int, window time.Duration) echo.MiddlewareFunc {
	return RateLimiterScoped("default", limit, window)
}

// RateLimiterScoped returns a rate limiter scoped to a specific subsystem (e.g. "auth_login", "share_unlock").
func RateLimiterScoped(scope string, limit int, window time.Duration) echo.MiddlewareFunc {
	statePath := os.Getenv("RATE_LIMITER_STATE_PATH")
	if statePath == "" {
		if dataDir := os.Getenv("DATA_DIR_PATH"); dataDir != "" {
			if scope == "" || scope == "default" {
				statePath = filepath.Join(dataDir, "db", "rate_limiter_state.json")
			} else {
				statePath = filepath.Join(dataDir, "db", fmt.Sprintf("rate_limiter_%s_state.json", scope))
			}
		}
	}
	return RateLimiterWithFile(limit, window, statePath)
}

// RateLimiterWithFile returns a rate limiter that persists buckets to a JSON file if statePath is provided.
// If reading existing state fails or writing the snapshot fails, it returns HTTP 503 (fail-closed).
func RateLimiterWithFile(limit int, window time.Duration, statePath string) echo.MiddlewareFunc {
	var mu sync.Mutex
	if os.Getenv("DISABLE_RATE_LIMIT") == "true" || os.Getenv("MY_STORAGE_E2E_DATA_DIR") != "" {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return next(c)
			}
		}
	}

	buckets := make(map[string]*ipBucket)
	var initErr error
	if statePath != "" {
		if data, err := os.ReadFile(statePath); err == nil {
			if err := json.Unmarshal(data, &buckets); err != nil {
				initErr = fmt.Errorf("corrupt rate limiter state: %w", err)
			}
		} else if !os.IsNotExist(err) {
			initErr = fmt.Errorf("read rate limiter state: %w", err)
		}
	}

	flush := func() error {
		if statePath == "" {
			return nil
		}
		data, err := json.Marshal(buckets)
		if err != nil {
			return fmt.Errorf("marshal rate limiter state: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
			return fmt.Errorf("mkdir rate limiter state: %w", err)
		}
		tempPath := statePath + ".tmp"
		if err := os.WriteFile(tempPath, data, 0600); err != nil {
			return fmt.Errorf("write rate limiter state: %w", err)
		}
		if err := os.Rename(tempPath, statePath); err != nil {
			return fmt.Errorf("rename rate limiter state: %w", err)
		}
		return nil
	}
	lastCleanup := time.Now()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			mu.Lock()
			if initErr != nil {
				mu.Unlock()
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "rate limiter state unavailable",
				})
			}

			ip := c.RealIP()
			if ip == "" {
				ip = "unknown"
			}

			now := time.Now()

			// Periodic cleanup of stale entries every 5 minutes
			if now.Sub(lastCleanup) > 5*time.Minute {
				cutoff := now.Add(-window)
				for k, b := range buckets {
					var valid []time.Time
					for _, ts := range b.Timestamps {
						if ts.After(cutoff) {
							valid = append(valid, ts)
						}
					}
					if len(valid) == 0 {
						delete(buckets, k)
					} else {
						b.Timestamps = valid
					}
				}
				lastCleanup = now
				if err := flush(); err != nil {
					mu.Unlock()
					return c.JSON(http.StatusServiceUnavailable, map[string]string{
						"error": "rate limiter state unavailable",
					})
				}
			}

			b, exists := buckets[ip]
			if !exists {
				b = &ipBucket{}
				buckets[ip] = b
			}

			// Filter out timestamps outside current window
			cutoff := now.Add(-window)
			var recent []time.Time
			for _, ts := range b.Timestamps {
				if ts.After(cutoff) {
					recent = append(recent, ts)
				}
			}
			b.Timestamps = recent

			if len(b.Timestamps) >= limit {
				oldest := b.Timestamps[0]
				retryAfter := int(window.Seconds() - now.Sub(oldest).Seconds() + 1)
				if retryAfter < 1 {
					retryAfter = 1
				}
				mu.Unlock()

				c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "too many requests, please try again later",
				})
			}

			b.Timestamps = append(b.Timestamps, now)
			if err := flush(); err != nil {
				mu.Unlock()
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "rate limiter state unavailable",
				})
			}
			mu.Unlock()

			return next(c)
		}
	}
}
