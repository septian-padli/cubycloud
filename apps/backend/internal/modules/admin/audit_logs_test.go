package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestGetAuditLogs(t *testing.T) {
	db, handler, _ := setupTestAdmin(t)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY, user_id TEXT, ip_address TEXT, user_agent TEXT,
		method TEXT NOT NULL, path TEXT NOT NULL, status_code INTEGER NOT NULL,
		action TEXT NOT NULL, details TEXT, error_message TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO audit_logs (id, user_id, method, path, status_code, action, created_at)
		VALUES ('audit-1', 'user-1', 'POST', '/api/v1/files', 201, 'POST /api/v1/files', '2026-01-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler.GetAuditLogs(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"audit-1"`) || !strings.Contains(rec.Body.String(), `"action":"POST /api/v1/files"`) {
 		t.Fatalf("response missing audit row: %s", rec.Body.String())
 	}
 }
