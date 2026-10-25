package httputil

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// IsSecureRequest returns true if the incoming request should be treated as secure
// for cookie generation and transport security. It evaluates:
// 1. Explicit COOKIE_SECURE=true environment variable
// 2. Direct TLS connection (r.TLS != nil)
// 3. Reverse proxy header (X-Forwarded-Proto: https, handling comma-separated chains)
func IsSecureRequest(r *http.Request) bool {
	if strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true") {
		return true
	}
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto != "" {
		if idx := strings.Index(proto, ","); idx != -1 {
			proto = proto[:idx]
		}
		if strings.EqualFold(strings.TrimSpace(proto), "https") {
			return true
		}
	}
	return false
}

// IsSecureCookie returns true if a cookie set on the given Echo context should have Secure: true.
func IsSecureCookie(c echo.Context) bool {
	if c == nil {
		return strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true")
	}
	return IsSecureRequest(c.Request())
}
