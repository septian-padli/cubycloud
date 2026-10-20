package setup

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// setupTestDB creates in-memory SQLite + runs migrations
func setupTestDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	sqldb.SetMaxOpenConns(1)
	if _, err := sqldb.Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("wal: %v", err)
	}
	if _, err := sqldb.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Run migrations inline (in-memory → no file migrations)
	ctx := context.Background()
	
	// users
	_, err = db.ExecContext(ctx, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			recovery_key_hash TEXT,
			role TEXT NOT NULL,
			quota_mode TEXT NOT NULL DEFAULT 'DEFAULT',
			quota_bytes INTEGER NOT NULL DEFAULT -1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("create users: %v", err)
	}

	// storage_nodes
	_, err = db.ExecContext(ctx, `
		CREATE TABLE storage_nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			path TEXT,
			endpoint TEXT,
			bucket TEXT,
			region TEXT,
			access_key_encrypted BLOB,
			secret_key_encrypted BLOB,
			capacity_bytes INTEGER NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("create storage_nodes: %v", err)
	}

	// settings
	_, err = db.ExecContext(ctx, `
		CREATE TABLE settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create settings: %v", err)
	}

	return db
}

func cleanupTestDB(t *testing.T, db *bun.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("close db: %v", err)
	}
}

// createTempDir creates temp dir for storage paths
func createTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cuby-test-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// mockS3Server returns httptest server mimicking S3 HeadBucket
func mockS3Server(t *testing.T, behavior string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	switch behavior {
	case "success":
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "HEAD" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
	case "auth_fail":
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
	case "timeout":
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Second)
		})
	default:
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	}

	return httptest.NewServer(mux)
}

// mockStorageDriver implements storage.StorageDriver for tests
type mockStorageDriver struct {
	validateErr error
}

func (m *mockStorageDriver) WriteLocal(ctx context.Context, nodePath string, filename string, reader io.Reader) error {
	return nil
}

func (m *mockStorageDriver) ReadLocal(ctx context.Context, nodePath string, filename string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockStorageDriver) DeleteLocal(ctx context.Context, nodePath string, filename string) error {
	return nil
}

func (m *mockStorageDriver) StatLocal(ctx context.Context, nodePath string) (capacity, free int64, err error) {
	return 1000000000, 500000000, nil
}

func (m *mockStorageDriver) GeneratePresignedUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	return "https://example.com/upload", nil
}

func (m *mockStorageDriver) GeneratePresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	return "https://example.com/download", nil
}

func (m *mockStorageDriver) ValidateCloudCredentials(ctx context.Context, endpoint, region, bucket, accessKey, secretKey string) error {
	return m.validateErr
}
