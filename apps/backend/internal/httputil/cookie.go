package httputil

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// IsSecureRequest returns true if the incoming request should be treated as secure
// for cookie generation and transport security. It evaluates:
// 1. Explicit COOKIE_SECURE environment variable (forces true or false)
// 2. Direct TLS connection (r.TLS != nil)
// 3. Reverse proxy header (X-Forwarded-Proto: https) ONLY when TRUST_PROXY=true
func IsSecureRequest(r *http.Request) bool {
	rawCookieSecure := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	if rawCookieSecure != "" {
		return strings.EqualFold(rawCookieSecure, "true")
	}
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TRUST_PROXY")), "true") {
		proto := r.Header.Get("X-Forwarded-Proto")
		if proto != "" {
			if idx := strings.Index(proto, ","); idx != -1 {
				proto = proto[:idx]
			}
			if strings.EqualFold(strings.TrimSpace(proto), "https") {
				return true
			}
		}
	}
	return false
}

// IsSecureCookie returns true if a cookie set on the given Echo context should have Secure: true.
func IsSecureCookie(c echo.Context) bool {
	if c == nil {
		rawCookieSecure := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
		return strings.EqualFold(rawCookieSecure, "true")
	}
	return IsSecureRequest(c.Request())
}
