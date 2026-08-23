package storage

import (
	"context"
	"io"
	"time"

	"github.com/uptrace/bun"
)

// StorageDriver abstracts local disk + S3-compatible cloud operations.
type StorageDriver interface {
	// Local disk operations
	WriteLocal(ctx context.Context, nodePath string, filename string, reader io.Reader) error
	ReadLocal(ctx context.Context, nodePath string, filename string) (io.ReadCloser, error)
	DeleteLocal(ctx context.Context, nodePath string, filename string) error
	StatLocal(ctx context.Context, nodePath string) (capacity, free int64, err error)

	// Cloud (S3-compatible) operations
	GeneratePresignedUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	GeneratePresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	ValidateCloudCredentials(ctx context.Context, endpoint, region, bucket, accessKey, secretKey string) error
}

// NewStorageDriver returns a StorageDriver wrapping LocalDriver.
// ponytail: db unused for now; will hold encrypted cloud credentials later.
func NewStorageDriver(db *bun.DB) StorageDriver {
	return NewLocalDriver("/app/data/storage")
}
