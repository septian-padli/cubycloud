package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"

	_ "github.com/glebarez/go-sqlite"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect opens SQLite with WAL mode, busy_timeout=5000, MaxOpenConns=1.
// Runs migrations from embedded migrations/ folder.
func Connect(dataDir string) (*bun.DB, error) {
	dbPath := filepath.Join(dataDir, "db", "cubycloud.db")
	return Open(context.Background(), dbPath)
}

// Open opens SQLite with specified path and runs migrations.
func Open(ctx context.Context, dbPath string) (*bun.DB, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000", dbPath)
	
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqldb.SetMaxOpenConns(1)
	sqldb.SetMaxIdleConns(1)
	sqldb.SetConnMaxLifetime(0)

	// Set WAL mode
	if _, err := sqldb.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := sqldb.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	if err := runMigrations(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func runMigrations(ctx context.Context, db *bun.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		content, err := migrationsFS.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", name, err)
		}
	}

	return nil
}
