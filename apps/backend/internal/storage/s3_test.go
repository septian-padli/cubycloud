package storage

import (
	"context"
	"testing"
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

// ponytail: real S3/R2 validation test needs live credentials — skip in CI; add integration test with testcontainers/localstack when container infra ready.
