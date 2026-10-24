package mystorage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
)

type uploadLimitDriver struct {
	capacity     int64
	free         int64
	bytesWritten int64
	deleted      bool
	nodePath     string
	filename     string
	readPath     string
}

func (d *uploadLimitDriver) WriteLocal(_ context.Context, nodePath, filename string, r io.Reader) error {
	d.nodePath = nodePath
	d.filename = filename
	n, err := io.Copy(io.Discard, r)
	d.bytesWritten += n
	return err
}

func (d *uploadLimitDriver) ReadLocal(_ context.Context, nodePath, _ string) (io.ReadCloser, error) {
	d.readPath = nodePath
	return io.NopCloser(strings.NewReader("stored")), nil
}

func (d *uploadLimitDriver) DeleteLocal(context.Context, string, string) error {
	d.deleted = true
	return nil
}

func (d *uploadLimitDriver) StatLocal(context.Context, string) (int64, int64, error) {
	return d.capacity, d.free, nil
}

func (*uploadLimitDriver) GeneratePresignedUploadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (*uploadLimitDriver) GeneratePresignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (*uploadLimitDriver) ValidateCloudCredentials(context.Context, string, string, string, string, string) error {
	return nil
}
type syncCloudTestDriver struct {
	*uploadLimitDriver
	actualSize    int64
	deleted       bool
	deleteErr     error
	deleteStarted chan struct{}
	deleteFinish  chan struct{}
	headCalls     int
}

func (d *syncCloudTestDriver) ReadObject(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (*syncCloudTestDriver) WriteObject(context.Context, string, string, io.Reader, int64) error {
	return nil
}

func (d *syncCloudTestDriver) DeleteObject(context.Context, string, string) error {
	d.deleted = true
	if d.deleteStarted != nil {
		close(d.deleteStarted)
		<-d.deleteFinish
	}
	return d.deleteErr
}

func (d *syncCloudTestDriver) HeadObject(context.Context, string, string) (int64, error) {
	d.headCalls++
	return d.actualSize, nil
}

func (d *syncCloudTestDriver) PresignUpload(_ context.Context, _ string, key string, _ time.Duration) (string, error) {
	return "https://upload.example.test/" + key, nil
}

func (*syncCloudTestDriver) PresignDownload(context.Context, string, string, string, time.Duration) (string, error) {
	return "", nil
}

func TestUploadIntentRejectsNegativeSize(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES('owner', 'owner', 'owner@example.test', 'hash', 'REGULAR', -1)`); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, &uploadLimitDriver{})
	if _, err = service.UploadIntent(context.Background(), "owner", "file.bin", -1, nil, false); !errors.Is(err, errInvalidSize) {
		t.Fatalf("UploadIntent error = %v, want %v", err, errInvalidSize)
	}
}

func TestSyncCloudEnforcesActualObjectSize(t *testing.T) {
	tests := []struct {
		name       string
		quota      int64
		capacity   int64
		actualSize int64
		wantErr    error
	}{
		{name: "actual size exceeds user quota", quota: 10, capacity: 1000, actualSize: 11, wantErr: errQuota},
		{name: "actual size exceeds node capacity", quota: -1, capacity: 100, actualSize: 96, wantErr: errStorageFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
				t.Fatal(err)
			}
			db, err := database.Connect(dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES('owner', 'owner', 'owner@example.test', 'hash', 'REGULAR', ?)`, tt.quota)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`INSERT INTO storage_nodes(id, type, capacity_bytes, enabled) VALUES('s3-node', 'S3', ?, 1)`, tt.capacity)
			if err != nil {
				t.Fatal(err)
			}
			expires := time.Now().Add(time.Hour)
			_, err = db.Exec(`INSERT INTO pending_cloud_uploads(id, owner_id, storage_node_id, filename, size_bytes, expires_at) VALUES('upload', 'owner', 's3-node', 'file.bin', ?, ?)`, tt.actualSize, expires)
			if err != nil {
				t.Fatal(err)
			}

			driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, actualSize: tt.actualSize}
			if tt.capacity == 100 {
				if _, err = storage.SelectPlacement(context.Background(), db, driver, 95); !errors.Is(err, storage.ErrPoolFull) {
					t.Fatalf("SelectPlacement error = %v, want %v for undersized node", err, storage.ErrPoolFull)
				}
			}
			service := NewService(db, driver)
			_, err = service.SyncCloud(context.Background(), "owner", "upload")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SyncCloud error = %v, want %v", err, tt.wantErr)
			}
			if driver.deleted {
				t.Fatal("retryable quota/capacity failure deleted S3 object")
			}
			var itemCount int
			if err = db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE id='upload'`).Scan(&itemCount); err != nil {
				t.Fatal(err)
			}
			if itemCount != 0 {
				t.Fatalf("metadata rows = %d, want 0", itemCount)
			}
			var pending int
			if err = db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='upload'`).Scan(&pending); err != nil {
				t.Fatal(err)
			}
			if pending != 1 {
				t.Fatalf("pending rows = %d, want 1 for retry", pending)
			}
		})
	}
}

func TestSyncCloudRejectsMismatchedObjectSize(t *testing.T) {
	for _, actual := range []int64{0, 2} {
		t.Run(fmt.Sprintf("actual_%d", actual), func(t *testing.T) {
			db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "cloud.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES('owner','owner','owner@example.test','hash','REGULAR',-1)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',1000,1)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES('upload','owner','node','file.bin',1,datetime('now','+1 hour'))`); err != nil {
				t.Fatal(err)
			}
			driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, actualSize: actual}
			_, err = NewService(db, driver).SyncCloud(context.Background(), "owner", "upload")
			if !errors.Is(err, errInvalidSize) {
				t.Fatalf("actual size %d: error = %v, want %v", actual, err, errInvalidSize)
			}
			httpErr, ok := respond(echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/sync-cloud", nil), httptest.NewRecorder()), err).(*echo.HTTPError)
			if !ok || httpErr.Code != http.StatusBadRequest {
				t.Fatalf("HTTP mismatch error = %v, want 400", httpErr)
			}
			if driver.deleted {
				t.Fatal("size mismatch deleted object needed for retry")
			}
			var items, pending int
			if err := db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE id='upload'`).Scan(&items); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='upload'`).Scan(&pending); err != nil {
				t.Fatal(err)
			}
			if items != 0 || pending != 1 {
				t.Fatalf("after mismatch: metadata=%d pending=%d, want 0 and 1", items, pending)
			}
		})
	}
}

func TestSyncCloudCommitFailurePreservesObjectAndIntent(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared_%t", shared), func(t *testing.T) {
			db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "cloud.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, owner := range []string{"owner", "other"} {
				if _, err := db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES(?,?,?,'hash','REGULAR',-1)`, owner, owner, owner+"@example.test"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',1000,1)`); err != nil {
				t.Fatal(err)
			}
			if shared {
				if _, err := db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id,modified_at) VALUES('shared','other','shared.bin','file',1,'upload','node',CURRENT_TIMESTAMP)`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES('upload','owner','node','file.bin',1,datetime('now','+1 hour'))`); err != nil {
				t.Fatal(err)
			}
			// Deferred FK failure occurs only at Commit, after metadata and intent deletion.
			if _, err := db.Exec(`CREATE TABLE commit_failure (owner_id TEXT REFERENCES users(id) DEFERRABLE INITIALLY DEFERRED)`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TRIGGER fail_cloud_commit AFTER INSERT ON my_storage_items BEGIN INSERT INTO commit_failure(owner_id) VALUES('missing-owner'); END`); err != nil {
				t.Fatal(err)
			}
			driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, actualSize: 1}
			service := NewService(db, driver)
			if _, err := service.SyncCloud(context.Background(), "owner", "upload"); err == nil {
				t.Fatal("commit should fail on deferred FK")
			}
			if driver.deleted {
				t.Fatal("failed commit deleted existing/new S3 object")
			}
			var items, pending int
			if err := db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE disk_key='upload'`).Scan(&items); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='upload'`).Scan(&pending); err != nil {
				t.Fatal(err)
			}
			wantItems := 0
			if shared {
				wantItems = 1
			}
			if items != wantItems || pending != 1 {
				t.Fatalf("after commit failure: metadata=%d pending=%d, want %d and 1", items, pending, wantItems)
			}
			if _, err := db.Exec(`DROP TRIGGER fail_cloud_commit`); err != nil {
				t.Fatal(err)
			}
			if _, err := service.SyncCloud(context.Background(), "owner", "upload"); err != nil {
				t.Fatalf("retry after DB recovers: %v", err)
			}
		})
	}
}

func TestUploadIntentOutlivesPresignedURLAndExpires(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "cloud.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES('owner','owner','owner@example.test','hash','REGULAR',-1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',1000,1)`); err != nil {
		t.Fatal(err)
	}
	driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, actualSize: 1}
	service := NewService(db, driver)
	intent, err := service.UploadIntent(context.Background(), "owner", "file.bin", 1, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	urlExpiry, err := time.Parse(time.RFC3339, intent.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	var intentExpiry time.Time
	if err := db.QueryRow(`SELECT expires_at FROM pending_cloud_uploads WHERE id=?`, intent.UploadID).Scan(&intentExpiry); err != nil {
		t.Fatal(err)
	}
	if !intentExpiry.After(urlExpiry.Add(time.Hour)) || intentExpiry.After(time.Now().Add(48*time.Hour)) {
		t.Fatalf("intent expiry %s must outlive URL expiry %s, but remain bounded", intentExpiry, urlExpiry)
	}
	if cleaned, err := service.CleanupExpiredPendingCloudUploads(context.Background()); err != nil || cleaned != 0 || driver.deleted {
		t.Fatalf("active intent cleanup = %d, %v, object deleted=%t; want 0, nil, false", cleaned, err, driver.deleted)
	}
	// Simulate a slow transfer whose signed URL has expired, but whose intent is active.
	if _, err := db.Exec(`UPDATE pending_cloud_uploads SET created_at=datetime('now','-1 hour') WHERE id=?`, intent.UploadID); err != nil {
		t.Fatal(err)
	}
	renewed, err := service.RenewUploadIntentURL(context.Background(), "owner", intent.UploadID)
	if err != nil || renewed.UploadID != intent.UploadID || renewed.Method != "PUT" || renewed.URL != "https://upload.example.test/"+intent.UploadID {
		t.Fatalf("renewed intent = %#v, %v", renewed, err)
	}
	if _, err = service.RenewUploadIntentURL(context.Background(), "other", intent.UploadID); !errors.Is(err, errNotFound) {
		t.Fatalf("renew intent for another owner = %v, want %v", err, errNotFound)
	}
	synced, err := service.SyncCloud(context.Background(), "owner", intent.UploadID)
	if err != nil {
		t.Fatalf("sync active intent after URL lifetime: %v", err)
	}
	recovered, err := service.SyncCloud(context.Background(), "owner", intent.UploadID)
	if err != nil || recovered.ID != synced.ID {
		t.Fatalf("retry sync cloud = %#v, %v; want the same item %q", recovered, err, synced.ID)
	}
	intent, err = service.UploadIntent(context.Background(), "owner", "later.bin", 1, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	driver.headCalls = 0
	if _, err := db.Exec(`UPDATE pending_cloud_uploads SET expires_at=datetime('now','-1 minute') WHERE id=?`, intent.UploadID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SyncCloud(context.Background(), "owner", intent.UploadID); !errors.Is(err, errNotFound) {
		t.Fatalf("expired intent synced: %v, want %v", err, errNotFound)
	}
	if driver.headCalls != 0 || driver.deleted {
		t.Fatalf("expired intent inspected or deleted S3 object before cleanup: head=%d deleted=%t", driver.headCalls, driver.deleted)
	}
	_, err = service.CleanupExpiredPendingCloudUploads(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !driver.deleted {
		t.Fatal("expired pending S3 object was not cleaned up")
	}
	var pending int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id=?`, intent.UploadID).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("expired pending uploads = %d, want 0", pending)
	}
}

func TestUploadIntentMetadataFinalizationFailurePreservesS3AndIntent(t *testing.T) {
	t.Run("quota exhaustion during finalization", func(t *testing.T) {
		db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "cloud.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES('owner','owner','owner@example.test','hash','REGULAR',100)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',1000,1)`); err != nil {
			t.Fatal(err)
		}
		driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, actualSize: 50}
		service := NewService(db, driver)

		intent, err := service.UploadIntent(context.Background(), "owner", "document.pdf", 50, nil, false)
		if err != nil {
			t.Fatalf("UploadIntent: %v", err)
		}

		// Simulate quota exhaustion before finalization
		if _, err := db.Exec(`UPDATE users SET quota_bytes=20 WHERE id='owner'`); err != nil {
			t.Fatal(err)
		}

		// Metadata finalization fails due to quota
		_, syncErr := service.SyncCloud(context.Background(), "owner", intent.UploadID)
		if !errors.Is(syncErr, errQuota) {
			t.Fatalf("SyncCloud err = %v, want %v", syncErr, errQuota)
		}
		if driver.deleted {
			t.Fatal("S3 object deleted when metadata finalization failed")
		}

		var pendingCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id=?`, intent.UploadID).Scan(&pendingCount); err != nil {
			t.Fatal(err)
		}
		if pendingCount != 1 {
			t.Fatalf("pending_cloud_uploads count = %d, want 1", pendingCount)
		}

		var itemCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE id=?`, intent.UploadID).Scan(&itemCount); err != nil {
			t.Fatal(err)
		}
		if itemCount != 0 {
			t.Fatalf("my_storage_items count = %d, want 0", itemCount)
		}

		// Quota increased/restored -> retry SyncCloud succeeds
		if _, err := db.Exec(`UPDATE users SET quota_bytes=100 WHERE id='owner'`); err != nil {
			t.Fatal(err)
		}
		item, err := service.SyncCloud(context.Background(), "owner", intent.UploadID)
		if err != nil {
			t.Fatalf("SyncCloud retry failed: %v", err)
		}
		if item.ID != intent.UploadID || item.SizeBytes != 50 {
			t.Fatalf("synced item mismatch: %+v", item)
		}
		if driver.deleted {
			t.Fatal("S3 object deleted after successful retry")
		}

		if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id=?`, intent.UploadID).Scan(&pendingCount); err != nil {
			t.Fatal(err)
		}
		if pendingCount != 0 {
			t.Fatalf("pending_cloud_uploads count = %d, want 0 after sync", pendingCount)
		}
	})

	t.Run("invalid parent folder during finalization", func(t *testing.T) {
		db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "cloud.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES('owner','owner','owner@example.test','hash','REGULAR',-1)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',1000,1)`); err != nil {
			t.Fatal(err)
		}
		// Create a parent folder
		if _, err := db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,storage_node_id,modified_at) VALUES('folder1','owner','Documents','folder',0,'node',CURRENT_TIMESTAMP)`); err != nil {
			t.Fatal(err)
		}

		parent := "folder1"
		driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, actualSize: 30}
		service := NewService(db, driver)

		intent, err := service.UploadIntent(context.Background(), "owner", "report.docx", 30, &parent, false)
		if err != nil {
			t.Fatalf("UploadIntent: %v", err)
		}

		// Parent folder is soft-deleted before finalization
		if _, err := db.Exec(`UPDATE my_storage_items SET deleted_at=CURRENT_TIMESTAMP WHERE id='folder1'`); err != nil {
			t.Fatal(err)
		}

		// Metadata finalization fails due to bad parent
		_, syncErr := service.SyncCloud(context.Background(), "owner", intent.UploadID)
		if !errors.Is(syncErr, errNotFound) {
			t.Fatalf("SyncCloud err = %v, want %v", syncErr, errNotFound)
		}
		if driver.deleted {
			t.Fatal("S3 object deleted when metadata finalization failed")
		}

		var pendingCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id=?`, intent.UploadID).Scan(&pendingCount); err != nil {
			t.Fatal(err)
		}
		if pendingCount != 1 {
			t.Fatalf("pending_cloud_uploads count = %d, want 1", pendingCount)
		}

		// Parent folder restored -> retry SyncCloud succeeds
		if _, err := db.Exec(`UPDATE my_storage_items SET deleted_at=NULL WHERE id='folder1'`); err != nil {
			t.Fatal(err)
		}
		item, err := service.SyncCloud(context.Background(), "owner", intent.UploadID)
		if err != nil {
			t.Fatalf("SyncCloud retry failed: %v", err)
		}
		if item.ID != intent.UploadID || item.ParentID == nil || *item.ParentID != "folder1" {
			t.Fatalf("synced item mismatch: %+v", item)
		}
		if driver.deleted {
			t.Fatal("S3 object deleted after successful retry")
		}

		if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id=?`, intent.UploadID).Scan(&pendingCount); err != nil {
			t.Fatal(err)
		}
		if pendingCount != 0 {
			t.Fatalf("pending_cloud_uploads count = %d, want 0 after sync", pendingCount)
		}
	})
}

func TestExpiredOrCancelledCloudIntentKeepsSharedObject(t *testing.T) {
	for _, source := range []string{"my_storage_items", "organization_items"} {
		for _, expired := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_expired_%t", source, expired), func(t *testing.T) {
				db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "cloud.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				for _, user := range []string{"owner", "other"} {
					if _, err := db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES(?,?,?,'hash','REGULAR',-1)`, user, user, user+"@example.test"); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',1000,1)`); err != nil {
					t.Fatal(err)
				}
				if source == "my_storage_items" {
					_, err = db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id,modified_at) VALUES('shared','other','shared.bin','file',1,'upload','node',CURRENT_TIMESTAMP)`)
				} else {
					_, err = db.Exec(`INSERT INTO organization_items(id,created_by,name,kind,size_bytes,disk_key,storage_node_id) VALUES('shared','other','shared.bin','file',1,'upload','node')`)
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES('upload','owner','node','file.bin',1,datetime('now','+1 hour'))`); err != nil {
					t.Fatal(err)
				}
				driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}}
				service := NewService(db, driver)
				if expired {
					if _, err := db.Exec(`UPDATE pending_cloud_uploads SET expires_at=datetime('now','-1 minute') WHERE id='upload'`); err != nil {
						t.Fatal(err)
					}
					n, err := service.CleanupExpiredPendingCloudUploads(context.Background())
					if err != nil || n != 1 {
						t.Fatalf("cleanup = %d, %v, want 1 and nil", n, err)
					}
				} else {
					deleted, err := service.CancelUploadIntent(context.Background(), "owner", "upload")
					if err != nil || !deleted {
						t.Fatalf("cancel = %t, %v, want true and nil", deleted, err)
					}
				}
				if driver.deleted {
					t.Fatal("cleanup/cancel deleted a referenced cloud object")
				}
				var items, pending int
				if err := db.QueryRow(`SELECT COUNT(*) FROM ` + source + ` WHERE disk_key='upload'`).Scan(&items); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='upload'`).Scan(&pending); err != nil {
					t.Fatal(err)
				}
				if items != 1 || pending != 0 {
					t.Fatalf("after cleanup/cancel: shared items=%d, pending=%d", items, pending)
				}
			})
		}
	}
}
func TestCancelUploadIntentSerializesWithSyncCloud(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES('owner', 'owner', 'owner@example.test', 'hash', 'REGULAR', -1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO storage_nodes(id, type, capacity_bytes, enabled) VALUES('s3-node', 'S3', 1000, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO pending_cloud_uploads(id, owner_id, storage_node_id, filename, size_bytes, expires_at) VALUES('upload', 'owner', 's3-node', 'file.bin', 1, datetime('now','+1 hour'))`); err != nil {
		t.Fatal(err)
	}

	driver := &syncCloudTestDriver{
		uploadLimitDriver: &uploadLimitDriver{},
		deleteStarted:     make(chan struct{}),
		deleteFinish:      make(chan struct{}),
	}
	cancelService := NewService(db, driver)
	syncService := NewService(db, driver)
	cancelResult := make(chan error, 1)
	go func() {
		_, cancelErr := cancelService.CancelUploadIntent(context.Background(), "owner", "upload")
		cancelResult <- cancelErr
	}()
	<-driver.deleteStarted

	syncStarted := make(chan struct{})
	syncResult := make(chan error, 1)
	go func() {
		close(syncStarted)
		_, syncErr := syncService.SyncCloud(context.Background(), "owner", "upload")
		syncResult <- syncErr
	}()
	<-syncStarted
	close(driver.deleteFinish)
	if err = <-cancelResult; err != nil {
		t.Fatalf("cancel upload intent: %v", err)
	}
	if err = <-syncResult; !errors.Is(err, errNotFound) {
		t.Fatalf("SyncCloud after cancellation = %v, want %v", err, errNotFound)
	}
	if driver.headCalls != 0 {
		t.Fatalf("SyncCloud inspected S3 after cancellation; HeadObject calls = %d", driver.headCalls)
	}
	var itemCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE id='upload'`).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 0 {
		t.Fatalf("metadata rows = %d after cancel/sync race, want 0", itemCount)
	}
}


func TestUploadBoundsStreamByQuotaAndNodeCapacity(t *testing.T) {
	tests := []struct {
		name             string
		quotaBytes       int64
		nodeCapacity     int64
		physicalCapacity int64
		physicalFree     int64
		wantError        string
		wantLimit        int64
	}{
		{name: "user quota", quotaBytes: 10, nodeCapacity: 5000, physicalCapacity: 1000, physicalFree: 900, wantError: "USER_QUOTA_EXCEEDED", wantLimit: 10},
		{name: "physical reserve", quotaBytes: -1, nodeCapacity: 5000, physicalCapacity: 1000, physicalFree: 900, wantError: "STORAGE_POOL_FULL", wantLimit: 4650},
		{name: "assigned node usage", quotaBytes: -1, nodeCapacity: 1000, physicalCapacity: 10000, physicalFree: 10000, wantError: "STORAGE_POOL_FULL", wantLimit: 850},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
				t.Fatal(err)
			}
			db, err := database.Connect(dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
				"user-id", "user", "user@example.test", "hash", "REGULAR", tt.quotaBytes)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, -1)`,
				"other-id", "other", "other@example.test", "hash", "REGULAR")
			if err != nil {
				t.Fatal(err)
			}
			nodePath := filepath.Join(dataDir, "storage", "default")
			_, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES(?,?,'LOCAL',?,?,1)`,
				"placement-node", "Default", nodePath, tt.nodeCapacity)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id,modified_at) VALUES(?,?,'used.bin','file',100,?,?,CURRENT_TIMESTAMP)`,
				"used-item", "other-id", "used-blob", "placement-node")
			if err != nil {
				t.Fatal(err)
			}

			driver := &uploadLimitDriver{capacity: tt.physicalCapacity, free: tt.physicalFree}
			handler := &Handler{service: NewService(db, driver)}
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "large.bin")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = part.Write(bytes.Repeat([]byte{'x'}, 1<<20)); err != nil {
				t.Fatal(err)
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}

			request := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/upload?filename=large.bin", &body)
			request.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
			context := echo.New().NewContext(request, httptest.NewRecorder())
			context.Set("user_id", "user-id")
			err = handler.upload(context)
			var httpErr *echo.HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("want HTTP error, got %v", err)
			}
			if httpErr.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusRequestEntityTooLarge)
			}
			message, ok := httpErr.Message.(map[string]string)
			if !ok || message["error"] != tt.wantError {
				t.Fatalf("error = %#v, want %q", httpErr.Message, tt.wantError)
			}
			if driver.bytesWritten != tt.wantLimit+1 {
				t.Fatalf("stored %d bytes, want bounded read of %d bytes", driver.bytesWritten, tt.wantLimit+1)
			}
			if !driver.deleted {
				t.Fatal("partial upload was not deleted")
			}
			var count int
			if err = db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE owner_id=?`, "user-id").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("stored %d file records after rejection, want none", count)
			}
		})
	}
}
func TestUploadPlacementDuplicateAndLegacyReadPath(t *testing.T) {
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
	if _, err = db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES('user-id','user','user@example.test','hash','REGULAR',-1)`); err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dataDir, "storage", "node")
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('node-id','Node','LOCAL',?,1073741824,1)`, nodePath); err != nil {
		t.Fatal(err)
	}
	driver := &uploadLimitDriver{}
	service := NewService(db, driver)
	limit, limitErr, err := service.UploadLimit(ctx, "user-id")
	if err != nil {
		t.Fatal(err)
	}
	item, err := service.Upload(ctx, "user-id", "placed.txt", nil, strings.NewReader("stored"), false, limit, limitErr)
	if err != nil {
		t.Fatal(err)
	}
	if driver.nodePath != nodePath {
		t.Fatalf("upload path = %q, want %q", driver.nodePath, nodePath)
	}
	var associated string
	if err = db.QueryRow(`SELECT storage_node_id FROM my_storage_items WHERE id=?`, item.ID).Scan(&associated); err != nil {
		t.Fatal(err)
	}
	if associated != "node-id" {
		t.Fatalf("storage_node_id = %q, want node-id", associated)
	}
	duplicate, err := service.Duplicate(ctx, "user-id", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT storage_node_id FROM my_storage_items WHERE id=?`, duplicate.ID).Scan(&associated); err != nil {
		t.Fatal(err)
	}
	if associated != "node-id" {
		t.Fatalf("duplicate storage_node_id = %q, want node-id", associated)
	}
	if _, err = db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,modified_at) VALUES('legacy-id','user-id','legacy.txt','file',6,'legacy-key',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, reader, err := service.Download(ctx, "user-id", "legacy-id"); err != nil {
		t.Fatal(err)
	} else {
		_ = reader.Close()
	}
	if driver.readPath != "" {
		t.Fatalf("legacy read path = %q, want base storage path", driver.readPath)
	}
}

func TestUpdateRejectsOversizedRequestBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/my-storage/item", strings.NewReader(strings.Repeat("x", (64<<10)+1)))
	context := echo.New().NewContext(request, httptest.NewRecorder())

	err := (&Handler{}).update(context)
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("update error = %v, want HTTP %d", err, http.StatusRequestEntityTooLarge)
	}
}

type blockingUploadDriver struct {
	*uploadLimitDriver
	started chan struct{}
	release chan struct{}
}

func (d *blockingUploadDriver) WriteLocal(ctx context.Context, nodePath, key string, r io.Reader) error {
	close(d.started)
	<-d.release
	return d.uploadLimitDriver.WriteLocal(ctx, nodePath, key, r)
}

func TestUploadDoesNotHoldDatabaseDuringStream(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, ?)`,
		"user-id", "user", "user@example.test", "hash", "REGULAR", -1); err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dataDir, "storage", "default")
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES('node-id','Default','LOCAL',?,1073741824,1)`, nodePath); err != nil {
		t.Fatal(err)
	}

	driver := &blockingUploadDriver{
		uploadLimitDriver: &uploadLimitDriver{capacity: 1 << 30, free: 1 << 30},
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	released := false
	release := func() {
		if !released {
			close(driver.release)
			released = true
		}
	}
	defer release()
	service := NewService(db, driver)
	ctx := context.Background()
	maxBytes, limitErr, err := service.UploadLimit(ctx, "user-id")
	if err != nil {
		t.Fatal(err)
	}
	uploadDone := make(chan error, 1)
	go func() {
		_, uploadErr := service.Upload(ctx, "user-id", "slow.txt", nil, strings.NewReader("slow upload"), false, maxBytes, limitErr)
		uploadDone <- uploadErr
	}()
	select {
	case <-driver.started:
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not reach storage write")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	var one int
	queryErr := db.QueryRowContext(queryCtx, "SELECT 1").Scan(&one)
	release()
	uploadErr := <-uploadDone
	if queryErr != nil {
		t.Fatalf("database query blocked while upload stream was stalled: %v", queryErr)
	}
	if one != 1 {
		t.Fatalf("query result = %d, want 1", one)
	}
	if uploadErr != nil {
		t.Fatalf("upload failed after stream resumed: %v", uploadErr)
	}
}

func TestOverQuotaOperations(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// User starts with quota of 50 bytes, but already has a 100-byte file (over quota)
	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_mode, quota_bytes) VALUES(?, ?, ?, ?, ?, 'CUSTOM', ?)`,
		"over-user", "overuser", "over@example.test", "hash", "REGULAR", 50)
	if err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(dataDir, "storage", "default")
	_, err = db.Exec(`INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES(?,?,'LOCAL',?,100000,1)`,
		"placement-node", "Default", nodePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id,modified_at) VALUES(?,?,'existing.bin','file',100,'disk-existing','placement-node',CURRENT_TIMESTAMP)`,
		"item-existing", "over-user")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,storage_node_id,modified_at) VALUES(?,?,'target_folder','folder',0,'placement-node',CURRENT_TIMESTAMP)`,
		"folder-target", "over-user")
	if err != nil {
		t.Fatal(err)
	}

	driver := &uploadLimitDriver{capacity: 100000, free: 100000}
	handler := &Handler{service: NewService(db, driver)}

	_, err = db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_mode, quota_bytes) VALUES(?, ?, ?, ?, ?, 'CUSTOM', ?)`,
		"super-admin", "superadmin", "super@example.test", "hash", "SUPER_ADMIN", 1)
	if err != nil {
		t.Fatal(err)
	}
	remaining, err := handler.service.quotaRemaining(context.Background(), db, "super-admin")
	if err != nil {
		t.Fatal(err)
	}
	if remaining <= 100 {
		t.Fatalf("super admin quota remaining = %d, want unlimited despite stored 1-byte quota", remaining)
	}

	// 1. Upload new file while over-quota must be rejected with 413 USER_QUOTA_EXCEEDED
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "newfile.bin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("some new content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drive/my-storage/upload?filename=newfile.bin", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	c := echo.New().NewContext(req, httptest.NewRecorder())
	c.Set("user_id", "over-user")
	err = handler.upload(c)
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("want HTTP error, got %v", err)
	}
	if httpErr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", httpErr.Code)
	}
	msg, _ := httpErr.Message.(map[string]string)
	if msg["error"] != "USER_QUOTA_EXCEEDED" {
		t.Fatalf("error = %v, want USER_QUOTA_EXCEEDED", msg["error"])
	}

	// 2. Listing items must succeed
	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage", nil)
	rec := httptest.NewRecorder()
	c = echo.New().NewContext(req, rec)
	c.Set("user_id", "over-user")
	if err := handler.list(c); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("list code = %d, want 200", rec.Code)
	}

	// 3. Move existing file must succeed
	moveReq := httptest.NewRequest(http.MethodPatch, "/api/v1/drive/items/item-existing/move", strings.NewReader(`{"target_folder_id":"folder-target"}`))
	moveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c = echo.New().NewContext(moveReq, httptest.NewRecorder())
	c.Set("user_id", "over-user")
	c.SetParamNames("id")
	c.SetParamValues("item-existing")
	if err := handler.MoveItem(c); err != nil {
		t.Fatalf("move failed: %v", err)
	}

	// 4. Delete existing file must succeed
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/drive/my-storage/item-existing", nil)
	c = echo.New().NewContext(delReq, httptest.NewRecorder())
	c.Set("user_id", "over-user")
	c.SetParamNames("id")
	c.SetParamValues("item-existing")
	if err := handler.delete(c); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestListLargestFiles(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	driver := &uploadLimitDriver{capacity: 10 * 1024 * 1024 * 1024, free: 10 * 1024 * 1024 * 1024}
	service := NewService(db, driver)
	handler := NewHandler(service)

	now := time.Now().UTC()
	userA := "user-alice"
	userB := "user-bob"

	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, role) VALUES (?, 'alice', 'alice@test.local', 'hash', 'REGULAR')`, userA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, role) VALUES (?, 'bob', 'bob@test.local', 'hash', 'REGULAR')`, userB)
	if err != nil {
		t.Fatal(err)
	}

	// Folders for Alice: Documents -> Reports
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('f-docs', ?, 'Documents', 'folder', 0, NULL, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	parentDocs := "f-docs"
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('f-reports', ?, 'Reports', 'folder', 0, ?, ?)`, userA, parentDocs, now)
	if err != nil {
		t.Fatal(err)
	}
	parentReports := "f-reports"

	// Files for Alice
	// 1. file-root-50m (50 MB at Root)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-1', ?, 'BigVideo.mp4', 'file', 52428800, NULL, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 2. file-docs-100m (100 MB in Documents)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-2', ?, 'Archive.zip', 'file', 104857600, ?, ?)`, userA, parentDocs, now)
	if err != nil {
		t.Fatal(err)
	}
	// 3. file-reports-20m (20 MB in Reports subfolder)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-3', ?, 'AnnualReport.pdf', 'file', 20971520, ?, ?)`, userA, parentReports, now)
	if err != nil {
		t.Fatal(err)
	}
	// 4. file-root-5m (5 MB at Root)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-4', ?, 'Notes.txt', 'file', 5242880, NULL, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 5. file-trash (200 MB, but deleted/in trash)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at, deleted_at) VALUES ('file-trash', ?, 'Trashed.iso', 'file', 209715200, NULL, ?, ?)`, userA, now, now)
	if err != nil {
		t.Fatal(err)
	}
	// 6. file-bob (500 MB, owned by Bob)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-bob', ?, 'BobFile.mkv', 'file', 524288000, NULL, ?)`, userB, now)
	if err != nil {
		t.Fatal(err)
	}

	// Test 1: Query largest files for Alice (all 4 active files)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage/largest-files?limit=10&offset=0", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", userA)
	if err := handler.largestFiles(c); err != nil {
		t.Fatalf("largestFiles failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp LargestFilesResult
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if resp.Total != 4 {
		t.Fatalf("total = %d, want 4", resp.Total)
	}
	if len(resp.Items) != 4 {
		t.Fatalf("items count = %d, want 4", len(resp.Items))
	}
	if resp.HasMore {
		t.Fatalf("expected has_more=false, got true")
	}

	// Verify order: Archive.zip (100MB) -> BigVideo.mp4 (50MB) -> AnnualReport.pdf (20MB) -> Notes.txt (5MB)
	expectedIDs := []string{"file-2", "file-1", "file-3", "file-4"}
	expectedLocations := []string{
		"My Storage / Documents",
		"My Storage",
		"My Storage / Documents / Reports",
		"My Storage",
	}

	for i, item := range resp.Items {
		if item.ID != expectedIDs[i] {
			t.Errorf("item[%d] ID = %s, want %s", i, item.ID, expectedIDs[i])
		}
		if item.Location != expectedLocations[i] {
			t.Errorf("item[%d] Location = %s, want %s", i, item.Location, expectedLocations[i])
		}
	}

	// Test 2: Pagination page 1 (limit=2, offset=0)
	p1Req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage/largest-files?limit=2&offset=0", nil)
	p1Rec := httptest.NewRecorder()
	p1Ctx := echo.New().NewContext(p1Req, p1Rec)
	p1Ctx.Set("user_id", userA)
	if err := handler.largestFiles(p1Ctx); err != nil {
		t.Fatal(err)
	}
	var p1Resp LargestFilesResult
	if err := json.Unmarshal(p1Rec.Body.Bytes(), &p1Resp); err != nil {
		t.Fatal(err)
	}
	if len(p1Resp.Items) != 2 || !p1Resp.HasMore || p1Resp.NextOffset != 2 {
		t.Fatalf("page 1 invalid: len=%d, has_more=%v, next_offset=%d", len(p1Resp.Items), p1Resp.HasMore, p1Resp.NextOffset)
	}
	if p1Resp.Items[0].ID != "file-2" || p1Resp.Items[1].ID != "file-1" {
		t.Errorf("page 1 unexpected items: %v", p1Resp.Items)
	}

	// Test 3: Pagination page 2 (limit=2, offset=2)
	p2Req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage/largest-files?limit=2&offset=2", nil)
	p2Rec := httptest.NewRecorder()
	p2Ctx := echo.New().NewContext(p2Req, p2Rec)
	p2Ctx.Set("user_id", userA)
	if err := handler.largestFiles(p2Ctx); err != nil {
		t.Fatal(err)
	}
	var p2Resp LargestFilesResult
	if err := json.Unmarshal(p2Rec.Body.Bytes(), &p2Resp); err != nil {
		t.Fatal(err)
	}
	if len(p2Resp.Items) != 2 || p2Resp.HasMore || p2Resp.NextOffset != 4 {
		t.Fatalf("page 2 invalid: len=%d, has_more=%v, next_offset=%d", len(p2Resp.Items), p2Resp.HasMore, p2Resp.NextOffset)
	}
	if p2Resp.Items[0].ID != "file-3" || p2Resp.Items[1].ID != "file-4" {
		t.Errorf("page 2 unexpected items: %v", p2Resp.Items)
	}
}

func TestCategoryUsage(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	driver := &uploadLimitDriver{capacity: 10 * 1024 * 1024 * 1024, free: 10 * 1024 * 1024 * 1024}
	service := NewService(db, driver)
	handler := NewHandler(service)

	now := time.Now().UTC()
	userA := "user-alice"
	userB := "user-bob"

	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, role) VALUES (?, 'alice', 'alice@test.local', 'hash', 'REGULAR')`, userA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, role) VALUES (?, 'bob', 'bob@test.local', 'hash', 'REGULAR')`, userB)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Video: 10 MB active
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at) VALUES ('v1', ?, 'clip.mp4', 'file', 10000000, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 2. Image: 5 MB active
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at) VALUES ('img1', ?, 'photo.png', 'file', 5000000, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 3. Document: 2 MB active
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at) VALUES ('doc1', ?, 'doc.pdf', 'file', 2000000, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 4. Archive: 8 MB active
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at) VALUES ('arc1', ?, 'backup.zip', 'file', 8000000, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 5. Other: 1 MB active (e.g. .mp3 audio or .bin)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at) VALUES ('oth1', ?, 'song.mp3', 'file', 1000000, ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	// 6. Trash file: 15 MB (deleted_at IS NOT NULL) - a .pdf that is in trash
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at, deleted_at) VALUES ('tr1', ?, 'old_manual.pdf', 'file', 15000000, ?, ?)`, userA, now, now)
	if err != nil {
		t.Fatal(err)
	}
	// 7. Duplicate blob: 4 MB active doc.docx + identical blob copy doc_copy.docx
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, modified_at) VALUES ('dup1', ?, 'doc.docx', 'file', 4000000, 'blob-dup', ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, disk_key, modified_at) VALUES ('dup2', ?, 'doc_copy.docx', 'file', 4000000, 'blob-dup', ?)`, userA, now)
	if err != nil {
		t.Fatal(err)
	}

	// 8. Bob's files (should NOT be included in Alice's usage)
	_, err = db.Exec(`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, modified_at) VALUES ('bob-file', ?, 'bob.mp4', 'file', 50000000, ?)`, userB, now)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage/category-usage", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", userA)

	if err := handler.categoryUsage(c); err != nil {
		t.Fatalf("categoryUsage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res CategoryUsageResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expectedTotal := int64(45000000)
	if res.TotalBytes != expectedTotal {
		t.Errorf("expected total bytes %d, got %d", expectedTotal, res.TotalBytes)
	}

	catMap := make(map[string]int64)
	var sumCatBytes int64
	for _, cat := range res.Categories {
		catMap[cat.ID] = cat.Bytes
		sumCatBytes += cat.Bytes
	}

	if sumCatBytes != res.TotalBytes {
		t.Errorf("sum of category bytes %d != total bytes %d", sumCatBytes, res.TotalBytes)
	}

	if catMap["video"] != 10000000 {
		t.Errorf("expected video 10000000, got %d", catMap["video"])
	}
	if catMap["image"] != 5000000 {
		t.Errorf("expected image 5000000, got %d", catMap["image"])
	}
	if catMap["document"] != 6000000 {
		t.Errorf("expected document 6000000, got %d", catMap["document"])
	}
	if catMap["archive"] != 8000000 {
		t.Errorf("expected archive 8000000, got %d", catMap["archive"])
	}
	if catMap["other"] != 1000000 {
		t.Errorf("expected other 1000000, got %d", catMap["other"])
	}
	if catMap["trash"] != 15000000 {
		t.Errorf("expected trash 15000000, got %d", catMap["trash"])
	}
}
func TestCancelUploadIntentIsOwnerScoped(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, user := range []string{"owner-1", "owner-2"} {
		if _, err := db.Exec(`INSERT INTO users(id, username, email, password_hash, role, quota_bytes) VALUES(?, ?, ?, ?, ?, -1)`,
			user, user, user+"@example.test", "hash", "REGULAR"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO storage_nodes(id,name,type,capacity_bytes,enabled) VALUES('node','Node','S3',1000,1)`); err != nil {
		t.Fatal(err)
	}
	for _, intent := range []struct{ id, owner string }{{"own-intent", "owner-1"}, {"other-intent", "owner-2"}} {
		if _, err := db.Exec(`INSERT INTO pending_cloud_uploads(id,owner_id,storage_node_id,filename,size_bytes,expires_at) VALUES(?,?,?,'file',1,datetime('now','+1 hour'))`,
			intent.id, intent.owner, "node"); err != nil {
			t.Fatal(err)
		}
	}

	e := echo.New()
	driver := &syncCloudTestDriver{uploadLimitDriver: &uploadLimitDriver{}, deleteErr: errors.New("S3 unavailable")}
	RegisterRoutes(e, db, driver, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", c.Request().Header.Get("X-User-ID"))
			return next(c)
		}
	})
	cancel := func(id, user string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodDelete, "/api/v1/drive/my-storage/upload-intent/"+id, nil)
		request.Header.Set("X-User-ID", user)
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)
		return response
	}
	response := cancel("own-intent", "owner-1")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed object deletion status=%d, want %d", response.Code, http.StatusInternalServerError)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='own-intent'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("failed object deletion removed intent: count=%d, want 1", count)
	}
	driver.deleteErr = nil
	response = cancel("own-intent", "owner-1")
	if response.Code != http.StatusNoContent {
		t.Fatalf("owner cancellation retry status=%d, want %d", response.Code, http.StatusNoContent)
	}
	response = cancel("other-intent", "owner-1")
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner cancellation status=%d, want %d", response.Code, http.StatusNotFound)
	}
	count = 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_cloud_uploads WHERE id='other-intent'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("cross-owner cancellation changed other user's intent: count=%d", count)
	}
}

func TestGetItemMetadata(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{
		`INSERT INTO users (id, username, email, password_hash, role) VALUES ('owner-1', 'alice', 'alice@test', 'hash', 'REGULAR')`,
		`INSERT INTO users (id, username, email, password_hash, role) VALUES ('owner-2', 'bob', 'bob@test', 'hash', 'REGULAR')`,
		`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('some-folder', 'owner-1', 'Projects', 'folder', 0, NULL, '2026-01-01T00:00:00Z')`,
		`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-1', 'owner-1', 'doc.pdf', 'file', 1234, NULL, '2026-01-01T00:00:00Z')`,
		`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, modified_at) VALUES ('file-nested', 'owner-1', 'sub.txt', 'file', 5678, 'some-folder', '2026-01-01T00:00:00Z')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	e := echo.New()
	RegisterRoutes(e, db, &uploadLimitDriver{}, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", c.Request().Header.Get("X-User-ID"))
			return next(c)
		}
	})

	// 1. Owner can fetch nested file metadata
	req := httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage/items/file-nested", nil)
	req.Header.Set("X-User-ID", "owner-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var item Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ID != "file-nested" || item.Name != "sub.txt" || item.SizeBytes != 5678 {
		t.Fatalf("unexpected item data: %+v", item)
	}

	// 2. Another user gets 404 Not Found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/drive/my-storage/items/file-nested", nil)
	req.Header.Set("X-User-ID", "owner-2")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for other owner, got %d", rec.Code)
	}
}
