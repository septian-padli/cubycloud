package mystorage

import (
	"bytes"
	"context"
	"errors"
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
		{name: "physical reserve", quotaBytes: -1, nodeCapacity: 5000, physicalCapacity: 1000, physicalFree: 900, wantError: "STORAGE_POOL_FULL", wantLimit: 850},
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
