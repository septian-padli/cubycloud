package database_test

import (
	"context"
	"os"
	"testing"

	"github.com/cubycloud/backend/internal/database"
)

func TestMigrationsCreateSchema(t *testing.T) {
	ctx := context.Background()
	tmpFile := "/tmp/test_cuby.db"
	defer os.Remove(tmpFile)

	db, err := database.Open(ctx, tmpFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()
	// Verify users table
	_, err = db.NewSelect().Table("users").Count(ctx)
	if err != nil {
		t.Errorf("users table not created: %v", err)
	}
	// Verify storage_nodes table
	_, err = db.NewSelect().Table("storage_nodes").Count(ctx)
	if err != nil {
		t.Errorf("storage_nodes table not created: %v", err)
	}

	// Verify settings table with default values
	var setting database.Setting
	err = db.NewSelect().Model(&setting).Where("key = ?", "is_setup_completed").Scan(ctx)
	if err != nil {
		t.Errorf("settings table not seeded: %v", err)
	}
	if setting.Value != "false" {
		t.Errorf("is_setup_completed = %q, want 'false'", setting.Value)
	}

	// Verify placement_strategy default
	err = db.NewSelect().Model(&setting).Where("key = ?", "placement_strategy").Scan(ctx)
	if err != nil {
		t.Errorf("placement_strategy not seeded: %v", err)
	}
	if setting.Value != "FILL_FIRST" {
		t.Errorf("placement_strategy = %q, want 'FILL_FIRST'", setting.Value)
	}
}

func TestSuperAdminQuotaMigrationRestoresUnlimited(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/cubycloud.db"
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO users(id, username, email, password_hash, role, quota_mode, quota_bytes)
		VALUES('super-admin', 'superadmin', 'superadmin@example.test', 'hash', 'SUPER_ADMIN', 'CUSTOM', 1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = '010_super_admin_unlimited_quota.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer db.Close()

	var quotaMode string
	var quotaBytes int64
	if err := db.QueryRowContext(ctx, `SELECT quota_mode, quota_bytes FROM users WHERE id = 'super-admin'`).Scan(&quotaMode, &quotaBytes); err != nil {
		t.Fatal(err)
	}
	if quotaMode != "UNLIMITED" || quotaBytes != -1 {
		t.Fatalf("super admin quota = (%q, %d), want (UNLIMITED, -1)", quotaMode, quotaBytes)
	}
}

func TestWALMode(t *testing.T) {
	ctx := context.Background()
	tmpFile := "/tmp/test_cuby_wal.db"
	defer os.Remove(tmpFile)
	defer os.Remove(tmpFile + "-wal")
	defer os.Remove(tmpFile + "-shm")

	db, err := database.Open(ctx, tmpFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var journalMode string
	err = db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode failed: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want 'wal'", journalMode)
	}
}

func TestForeignKeys(t *testing.T) {
	ctx := context.Background()
	tmpFile := t.TempDir() + "/test_cuby_fk.db"

	db, err := database.Open(ctx, tmpFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 1. Verify PRAGMA foreign_keys is 1
	var fkStatus int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkStatus); err != nil {
		t.Fatalf("PRAGMA foreign_keys failed: %v", err)
	}
	if fkStatus != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fkStatus)
	}

	// 2. Verify foreign key constraint enforcement & ON DELETE CASCADE
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE test_fk_parents (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE test_fk_children (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL REFERENCES test_fk_parents(id) ON DELETE CASCADE
		);
	`); err != nil {
		t.Fatalf("create test tables failed: %v", err)
	}

	// Inserting child without parent must fail
	if _, err := db.ExecContext(ctx, `INSERT INTO test_fk_children (id, parent_id) VALUES ('c1', 'p_none')`); err == nil {
		t.Fatalf("expected foreign key violation error inserting child with non-existent parent, got nil")
	}

	// Insert parent then child
	if _, err := db.ExecContext(ctx, `INSERT INTO test_fk_parents (id) VALUES ('p1')`); err != nil {
		t.Fatalf("insert parent failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO test_fk_children (id, parent_id) VALUES ('c1', 'p1')`); err != nil {
		t.Fatalf("insert child failed: %v", err)
	}

	// Delete parent - cascade should remove child
	if _, err := db.ExecContext(ctx, `DELETE FROM test_fk_parents WHERE id = 'p1'`); err != nil {
		t.Fatalf("delete parent failed: %v", err)
	}

	var childCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_fk_children WHERE id = 'c1'`).Scan(&childCount); err != nil {
		t.Fatalf("count child after cascade delete failed: %v", err)
	}
	if childCount != 0 {
		t.Fatalf("expected 0 child rows after CASCADE delete, got %d", childCount)
	}
}
