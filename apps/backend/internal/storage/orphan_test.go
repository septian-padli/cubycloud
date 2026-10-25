package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
)

type orphanTestDriver struct {
	StorageDriver
	cloudObjects []string
	deletedLocal []string
	deletedS3    []string
}

func (d *orphanTestDriver) DeleteLocal(_ context.Context, nodePath, filename string) error {
	d.deletedLocal = append(d.deletedLocal, filepath.Join(nodePath, filename))
	return os.Remove(filepath.Join(nodePath, filename))
}

func (d *orphanTestDriver) ListObjects(context.Context, string, string) ([]string, error) {
	return d.cloudObjects, nil
}

func (d *orphanTestDriver) DeleteObject(_ context.Context, _, key string) error {
	d.deletedS3 = append(d.deletedS3, key)
	return nil
}

func (d *orphanTestDriver) ReadObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (d *orphanTestDriver) WriteObject(context.Context, string, string, io.Reader, int64) error {
	return nil
}

func (d *orphanTestDriver) HeadObject(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (d *orphanTestDriver) PresignUpload(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (d *orphanTestDriver) PresignDownload(context.Context, string, string, string, time.Duration) (string, error) {
	return "", nil
}

func TestReconcileOrphanBlobs(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	localPath := filepath.Join(dataDir, "local")
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"orphan", "active", "trashed", "fresh"} {
		if err := os.WriteFile(filepath.Join(localPath, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		if name != "fresh" {
			old := time.Now().Add(-time.Hour)
			if err := os.Chtimes(filepath.Join(localPath, name), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('local','Local','LOCAL',?,1000,1)`, localPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,bucket,capacity_bytes,enabled) VALUES('cloud','Cloud','S3','bucket',1000,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users(id,username,email,password_hash,role) VALUES('user','user','user@example.test','hash','REGULAR')`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, key string
		deleted bool
	}{{"active", "active", false}, {"trashed", "trashed", true}} {
		var deleted any
		if item.deleted {
			deleted = time.Now()
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,disk_key,deleted_at) VALUES(?, 'user', ?, 'file', ?, ?)`, item.id, item.id, item.key, deleted); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES('pending-key','user','cloud','pending',0,datetime('now','+1 hour'))`); err != nil {
		t.Fatal(err)
	}

	driver := &orphanTestDriver{StorageDriver: NewLocalDriver(dataDir), cloudObjects: []string{"cloud-orphan", "active", "trashed", "pending-key", ".system"}}
	report, err := ReconcileOrphanBlobs(ctx, db, driver, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(driver.deletedLocal) != 1 || filepath.Base(driver.deletedLocal[0]) != "orphan" {
		t.Fatalf("deleted local = %v, want only orphan", driver.deletedLocal)
	}
	if len(driver.deletedS3) != 1 || driver.deletedS3[0] != "cloud-orphan" {
		t.Fatalf("deleted S3 = %v, want only cloud-orphan", driver.deletedS3)
	}
	if report.LocalDeleted != 1 || report.S3Deleted != 1 {
		t.Fatalf("deleted counts local=%d S3=%d, want 1 each", report.LocalDeleted, report.S3Deleted)
	}
	if _, err := os.Stat(filepath.Join(localPath, "fresh")); err != nil {
		t.Fatalf("fresh file should survive grace period: %v", err)
	}
}
