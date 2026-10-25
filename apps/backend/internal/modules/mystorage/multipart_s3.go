package mystorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/uptrace/bun"
)

// S3 multipart primitives for the direct presigned transfer flow.
//
// The shared StorageDriver (internal/storage) only exposes whole-object PUT/GET
// presigning, so the multipart intent endpoints need node-scoped multipart I/O
// (Create/Complete/Abort/ListParts + per-part upload presigning) here. The
// credential handling mirrors internal/storage/cloud.go nodeDriver exactly:
// encrypted storage_nodes credentials decrypted with storage.DecryptCredential
// and the same key material (DATA_DIR_PATH, else the directory above the SQLite
// file). Provider secrets never leave the server; only short-lived part URLs do.
//
// This file intentionally adds no new dependency: aws-sdk-go-v2/service/s3 is
// already used by the storage package.

// ProviderPart is one uploaded part as reported by the provider.
type ProviderPart struct {
	PartNumber int32
	ETag       string
	SizeBytes  int64
}

// S3MultipartClient issues node-scoped S3 multipart operations without ever
// exposing node credentials to callers.
type S3MultipartClient struct {
	db *bun.DB

	keyOnce      sync.Once
	keyMaterial  []byte
	keyMaterialErr error
}

// NewS3MultipartClient returns a client for the multipart intent flow.
func NewS3MultipartClient(db *bun.DB) *S3MultipartClient {
	return &S3MultipartClient{db: db}
}

// keyForNodes resolves the credential-decrypt key material once, matching
// internal/storage/cloud.go nodeDriver: DATA_DIR_PATH env, else the directory
// that holds the SQLite file (the parent of ...) — the same value the driver
// derives from its storage base path.
func (c *S3MultipartClient) keyMaterialForNodes(ctx context.Context) ([]byte, error) {
	c.keyOnce.Do(func() {
		if dir := os.Getenv("DATA_DIR_PATH"); dir != "" {
			c.keyMaterial = []byte(dir)
			return
		}
		var dbFile string
		err := c.db.QueryRowContext(ctx, `SELECT "file" FROM pragma_database_list WHERE "name"='main'`).Scan(&dbFile)
		if err != nil {
			c.keyMaterialErr = fmt.Errorf("resolve data dir for credentials: %w", err)
			return
		}
		if dbFile == "" {
			c.keyMaterialErr = errors.New("resolve data dir for credentials: sqlite main path unavailable")
			return
		}
		c.keyMaterial = []byte(filepath.Dir(filepath.Dir(dbFile)))
	})
	return c.keyMaterial, c.keyMaterialErr
}

func (c *S3MultipartClient) client(ctx context.Context, nodeID string) (*s3.Client, string, error) {
	var endpoint, region, bucket string
	var accessEncrypted, secretEncrypted []byte
	err := c.db.QueryRowContext(ctx, `SELECT COALESCE(endpoint,''),COALESCE(region,''),COALESCE(bucket,''),access_key_encrypted,secret_key_encrypted FROM storage_nodes WHERE id=? AND type='S3'`, nodeID).Scan(&endpoint, &region, &bucket, &accessEncrypted, &secretEncrypted)
	if err != nil {
		return nil, "", err
	}
	keyMaterial, err := c.keyMaterialForNodes(ctx)
	if err != nil {
		return nil, "", err
	}
	access, err := storage.DecryptCredential(accessEncrypted, keyMaterial)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt cloud access key: %w", err)
	}
	secret, err := storage.DecryptCredential(secretEncrypted, keyMaterial)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt cloud secret key: %w", err)
	}
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(string(access), string(secret), "")))
	if err != nil {
		return nil, "", err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = true
	}), bucket, nil
}

// CreateMultipartUpload starts a provider-side multipart session for one object key.
func (c *S3MultipartClient) CreateMultipartUpload(ctx context.Context, nodeID, key string) (string, error) {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return "", err
	}
	out, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return "", err
	}
	if out.UploadId == nil || *out.UploadId == "" {
		return "", errors.New("provider returned no multipart upload id")
	}
	return *out.UploadId, nil
}

// PresignUploadPart returns a short-lived URL the browser PUTs one raw part to.
func (c *S3MultipartClient) PresignUploadPart(ctx context.Context, nodeID, key, uploadID string, partNumber int32, expiry time.Duration) (string, error) {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return "", err
	}
	part := partNumber
	presigner := s3.NewPresignClient(client)
	out, err := presigner.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     &bucket,
		Key:        &key,
		UploadId:   &uploadID,
		PartNumber: &part,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// ListParts reconciles every part the provider currently holds for the session.
func (c *S3MultipartClient) ListParts(ctx context.Context, nodeID, key, uploadID string) ([]ProviderPart, error) {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	input := &s3.ListPartsInput{Bucket: &bucket, Key: &key, UploadId: &uploadID}
	paginator := s3.NewListPartsPaginator(client, input)
	parts := make([]ProviderPart, 0, 8)
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range out.Parts {
			parts = append(parts, ProviderPart{
				PartNumber: aws.ToInt32(p.PartNumber),
				ETag:       aws.ToString(p.ETag),
				SizeBytes:  aws.ToInt64(p.Size),
			})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	return parts, nil
}

// CompleteMultipartUpload assembles the object from exactly the given parts
// (must be ascending part numbers with the provider ETag of each part).
func (c *S3MultipartClient) CompleteMultipartUpload(ctx context.Context, nodeID, key, uploadID string, parts []ProviderPart) error {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return err
	}
	completed := make([]s3types.CompletedPart, 0, len(parts))
	for _, p := range parts {
		part := p.PartNumber
		etag := p.ETag
		completed = append(completed, s3types.CompletedPart{PartNumber: &part, ETag: &etag})
	}
	sort.Slice(completed, func(i, j int) bool { return aws.ToInt32(completed[i].PartNumber) < aws.ToInt32(completed[j].PartNumber) })
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          &bucket,
		Key:             &key,
		UploadId:        &uploadID,
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: completed},
	})
	return err
}

// AbortMultipartUpload discards the provider-side multipart session; every
// already-uploaded part is dropped by the provider.
func (c *S3MultipartClient) AbortMultipartUpload(ctx context.Context, nodeID, key, uploadID string) error {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return err
	}
	_, err = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: &bucket, Key: &key, UploadId: &uploadID})
	return err
}

// HeadObjectSize returns the current object size, used to recover a session
// whose parts list is gone but whose object already exists.
func (c *S3MultipartClient) HeadObjectSize(ctx context.Context, nodeID, key string) (int64, error) {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return -1, err
	}
	out, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return -1, err
	}
	return aws.ToInt64(out.ContentLength), nil
}

// DeleteObject removes an unreferenced orphan object during abort/expiry cleanup.
func (c *S3MultipartClient) DeleteObject(ctx context.Context, nodeID, key string) error {
	client, bucket, err := c.client(ctx, nodeID)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &key})
	return err
}

// IsNoSuchUpload reports whether the provider dropped the multipart session
// (completed, aborted, or expired). Callers must reconcile against the object
// itself instead of silently restarting the upload.
func IsNoSuchUpload(err error) bool {
	var noSuch *s3types.NoSuchUpload
	return errors.As(err, &noSuch)
}
