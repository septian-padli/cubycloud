package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type LocalDriver struct {
	basePath string
}

func NewLocalDriver(basePath string) *LocalDriver {
	return &LocalDriver{basePath: basePath}
}

func (d *LocalDriver) localPath(nodePath, filename string) string {
	root := d.basePath
	if filepath.IsAbs(nodePath) {
		root = nodePath
	} else if nodePath != "" {
		basePath := filepath.Clean(d.basePath)
		nodePath = filepath.Clean(nodePath)
		if nodePath == basePath || strings.HasPrefix(nodePath, basePath+string(os.PathSeparator)) {
			root = nodePath
		} else {
			root = filepath.Join(root, nodePath)
		}
	}
	return filepath.Join(root, filename)
}

func (d *LocalDriver) WriteLocal(ctx context.Context, nodePath, filename string, reader io.Reader) error {
	fullPath := d.localPath(nodePath, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, reader)
	return err
}

func (d *LocalDriver) ReadLocal(ctx context.Context, nodePath, filename string) (io.ReadCloser, error) {
	return os.Open(d.localPath(nodePath, filename))
}

func (d *LocalDriver) DeleteLocal(ctx context.Context, nodePath, filename string) error {
	return os.Remove(d.localPath(nodePath, filename))
}

func (d *LocalDriver) StatLocal(ctx context.Context, nodePath string) (capacity, free int64, err error) {
	fullPath := d.localPath(nodePath, ".")
	return statfsCapacity(fullPath)
}

func (d *LocalDriver) GeneratePresignedUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	return "", fmt.Errorf("cloud not configured")
}

func (d *LocalDriver) GeneratePresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	return "", fmt.Errorf("cloud not configured")
}

func (d *LocalDriver) ValidateCloudCredentials(ctx context.Context, endpoint, region, bucket, accessKey, secretKey string) error {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = true
	})

	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket})
	return err
}
