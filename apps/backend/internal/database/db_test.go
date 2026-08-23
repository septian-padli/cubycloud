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
