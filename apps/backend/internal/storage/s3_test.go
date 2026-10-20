package storage
import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"github.com/cubycloud/backend/internal/database"
)

func TestValidateCloudCredentialsInvalid(t *testing.T) {
	d := NewStorageDriver(nil)
	ctx := context.Background()

	// Invalid credentials should fail HeadBucket
	err := d.ValidateCloudCredentials(ctx, "https://fake.endpoint", "us-east-1", "fake-bucket", "FAKE_KEY", "FAKE_SECRET")
	if err == nil {
		t.Fatal("expected error with invalid credentials")
	}
}
func TestWriteObjectUnknownSizeUsesS3CompatibleStreamingRequest(t *testing.T) {
	var contentLength string
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentLength = r.Header.Get("Content-Length")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	key := []byte(filepath.Clean(dataDir))
	access, err := EncryptCredential([]byte("access"), key)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := EncryptCredential([]byte("secret"), key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO storage_nodes(id,type,endpoint,bucket,region,access_key_encrypted,secret_key_encrypted,capacity_bytes,enabled)
		VALUES('s3-node','S3',?,'bucket','us-east-1',?,?,1000,1)`, server.URL, access, secret)
	if err != nil {
		t.Fatal(err)
	}

	driver := newNodeDriver(db, filepath.Join(dataDir, "storage")).(*nodeDriver)
	content := []byte("streamed payload")
	if err = driver.WriteObject(context.Background(), "s3-node", "object", bytes.NewReader(content), -1); err != nil {
		t.Fatalf("WriteObject unknown size: %v", err)
	}
	if contentLength == "-1" {
		t.Fatalf("S3 request sent invalid Content-Length %q", contentLength)
	}
	if !bytes.Equal(body, content) {
		t.Fatalf("S3 request body = %q, want %q", body, content)
	}
}

// ponytail: real S3/R2 validation test needs live credentials — skip in CI; add integration test with testcontainers/localstack when container infra ready.
