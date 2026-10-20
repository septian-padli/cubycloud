package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type ipBucket struct {
	timestamps []time.Time
}

// RateLimiter returns an Echo middleware that limits requests by client IP.
// limit: maximum requests allowed within window.
// window: duration window for the rate limit.
func RateLimiter(limit int, window time.Duration) echo.MiddlewareFunc {
	var mu sync.Mutex
	buckets := make(map[string]*ipBucket)
	lastCleanup := time.Now()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if ip == "" {
				ip = "unknown"
			}

			now := time.Now()
			mu.Lock()

			// Periodic cleanup of stale entries every 5 minutes
			if now.Sub(lastCleanup) > 5*time.Minute {
				cutoff := now.Add(-window)
				for k, b := range buckets {
					var valid []time.Time
					for _, ts := range b.timestamps {
						if ts.After(cutoff) {
							valid = append(valid, ts)
						}
					}
					if len(valid) == 0 {
						delete(buckets, k)
					} else {
						b.timestamps = valid
					}
				}
				lastCleanup = now
			}

			b, exists := buckets[ip]
			if !exists {
				b = &ipBucket{}
				buckets[ip] = b
			}

			// Filter out timestamps outside current window
			cutoff := now.Add(-window)
			var recent []time.Time
			for _, ts := range b.timestamps {
				if ts.After(cutoff) {
					recent = append(recent, ts)
				}
			}
			b.timestamps = recent

			if len(b.timestamps) >= limit {
				oldest := b.timestamps[0]
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

			b.timestamps = append(b.timestamps, now)
			mu.Unlock()

			return next(c)
		}
	}
}
