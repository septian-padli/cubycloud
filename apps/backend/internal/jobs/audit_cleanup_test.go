package jobs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
)

func TestPurgeExpiredAuditLogs(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES ('audit_retention_days', '7')`); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -8).Format("2006-01-02 15:04:05")
	fresh := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02 15:04:05")
	for _, row := range []struct{ id, created string }{{"old", old}, {"fresh", fresh}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO audit_logs(id, method, path, status_code, action, created_at) VALUES (?, 'POST', '/', 200, 'POST /', ?)`, row.id, row.created); err != nil {
			t.Fatal(err)
		}
	}
	deleted, err := PurgeExpiredAuditLogs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("remaining rows = %d, want 1", count)
	}
}
