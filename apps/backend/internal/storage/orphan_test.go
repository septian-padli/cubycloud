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
	listPrefix   string
	deletedLocal []string
	deletedS3    []string
}

func (d *orphanTestDriver) DeleteLocal(_ context.Context, nodePath, filename string) error {
	d.deletedLocal = append(d.deletedLocal, filepath.Join(nodePath, filename))
	return os.Remove(filepath.Join(nodePath, filename))
}

func (d *orphanTestDriver) ListObjects(_ context.Context, _, prefix string) ([]string, error) {
	d.listPrefix = prefix
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

func TestReconcileOrphanBlobsDryRunAndConfinedNamespace(t *testing.T) {
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
	instanceID, err := InstanceID(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	localPath := filepath.Join(dataDir, "local")
	ownedKey := BuildBlobKey(instanceID, "local", "blob")
	owned := filepath.Join(localPath, filepath.FromSlash(ownedKey))
	if err := os.MkdirAll(filepath.Dir(owned), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(owned, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(localPath, "unowned")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('local','Local','LOCAL',?,1000,1)`, localPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,bucket,capacity_bytes,enabled) VALUES('cloud','Cloud','S3','bucket',1000,1)`); err != nil {
		t.Fatal(err)
	}
	cloudOrphan := BuildBlobKey(instanceID, "cloud", "orphan")
	driver := &orphanTestDriver{StorageDriver: NewLocalDriver(dataDir), cloudObjects: []string{cloudOrphan}}

	// The default mode reports and records candidates, but never deletes.
	if _, err := ReconcileOrphanBlobs(ctx, db, driver, time.Hour); err != nil {
		t.Fatal(err)
	}
	if len(driver.deletedLocal) != 0 || len(driver.deletedS3) != 0 {
		t.Fatalf("default mode deleted local=%v s3=%v", driver.deletedLocal, driver.deletedS3)
	}
	if want := "cubycloud/" + instanceID + "/cloud/blobs/"; driver.listPrefix != want {
		t.Fatalf("S3 prefix = %q, want %q", driver.listPrefix, want)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
	var candidates int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orphan_candidates`).Scan(&candidates); err != nil || candidates != 2 {
		t.Fatalf("candidate count = %d, %v; want both owned orphans", candidates, err)
	}

	// A single observation and recent first_seen_at are insufficient for deletion.
	if _, err := ReconcileOrphanBlobs(ctx, db, driver, time.Hour, false); err != nil {
		t.Fatal(err)
	}
	if len(driver.deletedLocal) != 0 || len(driver.deletedS3) != 0 {
		t.Fatalf("deleted before grace: local=%v s3=%v", driver.deletedLocal, driver.deletedS3)
	}
	old := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := db.ExecContext(ctx, `UPDATE orphan_candidates SET first_seen_at=?,last_seen_at=?,observation_count=1`, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileOrphanBlobs(ctx, db, driver, time.Hour, false); err != nil {
		t.Fatal(err)
	}
	if len(driver.deletedLocal) != 1 || driver.deletedLocal[0] != owned {
		t.Fatalf("deleted local = %v, want only owned path %q", driver.deletedLocal, owned)
	}
	if len(driver.deletedS3) != 1 || driver.deletedS3[0] != cloudOrphan {
		t.Fatalf("deleted S3 = %v, want only %q", driver.deletedS3, cloudOrphan)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}

func TestReconcileOrphanBlobsProtectsActiveUploadsAndJobs(t *testing.T) {
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
	instanceID, err := InstanceID(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,bucket,capacity_bytes,enabled) VALUES('cloud','Cloud','S3','bucket',1000,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users(id,username,email,password_hash,role) VALUES('user','user','user@example.test','hash','REGULAR')`); err != nil {
		t.Fatal(err)
	}
	activeItem := BuildBlobKey(instanceID, "cloud", "active")
	if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,disk_key,storage_node_id) VALUES('item','user','item','file',?,'cloud')`, activeItem); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES('pending','user','cloud','pending',0,datetime('now','+1 hour'))`); err != nil {
		t.Fatal(err)
	}
	pendingKey := BuildBlobKey(instanceID, "cloud", "pending")
	if _, err := db.ExecContext(ctx, `INSERT INTO multipart_uploads(id,scope,owner_id,storage_node_id,s3_key,s3_upload_id,filename,size_bytes,part_size_bytes,part_count,expires_at) VALUES('multi','my-storage','user','cloud',?,'provider','multi',0,1,1,datetime('now','+1 hour'))`, BuildBlobKey(instanceID, "cloud", "multipart")); err != nil {
		t.Fatal(err)
	}
	driver := &orphanTestDriver{StorageDriver: NewLocalDriver(dataDir), cloudObjects: []string{activeItem, pendingKey, BuildBlobKey(instanceID, "cloud", "multipart"), BuildBlobKey(instanceID, "cloud", "orphan")}}
	if _, err := ReconcileOrphanBlobs(ctx, db, driver, time.Hour, false); err != nil {
		t.Fatal(err)
	}
	if len(driver.deletedS3) != 0 {
		t.Fatalf("deleted with fresh candidates: %v", driver.deletedS3)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO jobs(id,type,status,user_id,payload) VALUES('migration','node_migration','PROCESSING','user','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileOrphanBlobs(ctx, db, driver, time.Hour, false); err == nil {
		t.Fatal("expected active migration to fail closed")
	}
	if len(driver.deletedS3) != 0 {
		t.Fatalf("deleted while migration active: %v", driver.deletedS3)
	}
}
