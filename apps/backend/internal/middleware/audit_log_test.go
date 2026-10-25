package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cubycloud/backend/internal/database"
	"github.com/labstack/echo/v4"
)

func TestAuditLogMiddleware(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		e := echo.New()
		req := httptest.NewRequest(method, "/api/v1/test", strings.NewReader(`{"password":"p","nested":{"secret":"s"},"name":"ok"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", "user-1")
		mw := AuditLog(db)
		err := mw(func(c echo.Context) error {
			body, readErr := io.ReadAll(c.Request().Body)
			if readErr != nil || len(body) == 0 {
				t.Fatalf("request body unavailable after audit capture: %v", readErr)
			}
			c.Response().WriteHeader(http.StatusBadRequest)
			return nil
		})(c)
		if err != nil {
			t.Fatal(err)
		}
		var status int
		var details string
		if err := db.QueryRowContext(context.Background(), `SELECT status_code, details FROM audit_logs WHERE method=? ORDER BY created_at DESC LIMIT 1`, method).Scan(&status, &details); err != nil {
			t.Fatal(err)
		}
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if strings.Contains(details, `"p"`) || strings.Contains(details, `"s"`) || !strings.Contains(details, "[REDACTED]") {
			t.Fatalf("details not redacted: %s", details)
		}
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	if err := AuditLog(db)(func(echo.Context) error { return nil })(c); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM audit_logs WHERE method = 'GET'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("GET generated %d audit rows", count)
	}
}
