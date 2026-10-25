package storage
import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/uptrace/bun"
)

// CloudStorageDriver provides node-scoped cloud I/O without exposing node credentials.
type CloudStorageDriver interface {
	StorageDriver
	ReadObject(context.Context, string, string) (io.ReadCloser, error)
	WriteObject(context.Context, string, string, io.Reader, int64) error
	DeleteObject(context.Context, string, string) error
	HeadObject(context.Context, string, string) (int64, error)
	ListObjects(context.Context, string, string) ([]string, error)
	PresignUpload(context.Context, string, string, time.Duration) (string, error)
	PresignDownload(context.Context, string, string, string, time.Duration) (string, error)
}

func (d *nodeDriver) ListObjects(ctx context.Context, nodeID, prefix string) ([]string, error) {
	c, b, err := d.client(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	paginator := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{Bucket: &b, Prefix: aws.String(prefix)})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}
	return keys, nil
}

type nodeDriver struct {
 *LocalDriver
 db *bun.DB
 keyMaterial []byte
}

func newNodeDriver(db *bun.DB, path string) StorageDriver {
 km := []byte(filepath.Dir(path))
	if envDir := os.Getenv("DATA_DIR_PATH"); envDir != "" {
		km = []byte(filepath.Clean(envDir))
	}
 return &nodeDriver{LocalDriver: NewLocalDriver(path), db: db, keyMaterial: km}
}

func (d *nodeDriver) client(ctx context.Context, nodeID string) (*s3.Client, string, error) {
 var endpoint, region, bucket string
 var accessEncrypted, secretEncrypted []byte
 err := d.db.QueryRowContext(ctx, `SELECT COALESCE(endpoint,''),COALESCE(region,''),COALESCE(bucket,''),access_key_encrypted,secret_key_encrypted FROM storage_nodes WHERE id=? AND type='S3'`, nodeID).Scan(&endpoint,&region,&bucket,&accessEncrypted,&secretEncrypted)
 if err != nil { return nil,"",err }
 access,err := DecryptCredential(accessEncrypted,d.keyMaterial); if err != nil { return nil,"",fmt.Errorf("decrypt cloud access key: %w",err) }
 secret,err := DecryptCredential(secretEncrypted,d.keyMaterial); if err != nil { return nil,"",fmt.Errorf("decrypt cloud secret key: %w",err) }
 cfg,err := config.LoadDefaultConfig(ctx,config.WithRegion(region),config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(string(access),string(secret),"")))
 if err != nil { return nil,"",err }
 return s3.NewFromConfig(cfg,func(o *s3.Options){ if endpoint!="" { o.BaseEndpoint=aws.String(endpoint) }; o.UsePathStyle=true }),bucket,nil
}

func (d *nodeDriver) ReadObject(ctx context.Context,nodeID,key string)(io.ReadCloser,error){
 c,b,err:=d.client(ctx,nodeID); if err!=nil{return nil,err}; out,err:=c.GetObject(ctx,&s3.GetObjectInput{Bucket:&b,Key:&key}); if err!=nil{return nil,err}; return out.Body,nil
}
func (d *nodeDriver) WriteObject(ctx context.Context, nodeID, key string, r io.Reader, size int64) error {
	c, b, err := d.client(ctx, nodeID)
	if err != nil {
		return err
	}
	input := &s3.PutObjectInput{Bucket: &b, Key: &key, Body: r}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	_, err = c.PutObject(ctx, input)
	return err
}
func (d *nodeDriver) DeleteObject(ctx context.Context,nodeID,key string)error{
 c,b,err:=d.client(ctx,nodeID); if err!=nil{return err}; _,err=c.DeleteObject(ctx,&s3.DeleteObjectInput{Bucket:&b,Key:&key});return err
}
func (d *nodeDriver) HeadObject(ctx context.Context,nodeID,key string)(int64,error){
 c,b,err:=d.client(ctx,nodeID); if err!=nil{return 0,err}; out,err:=c.HeadObject(ctx,&s3.HeadObjectInput{Bucket:&b,Key:&key}); if err!=nil{return 0,err};return aws.ToInt64(out.ContentLength),nil
}
func (d *nodeDriver) PresignUpload(ctx context.Context,nodeID,key string,expiry time.Duration)(string,error){
 c,b,err:=d.client(ctx,nodeID); if err!=nil{return "",err};out,err:=s3.NewPresignClient(c).PresignPutObject(ctx,&s3.PutObjectInput{Bucket:&b,Key:&key},s3.WithPresignExpires(expiry));if err!=nil{return "",err};return out.URL,nil
}
func (d *nodeDriver) PresignDownload(ctx context.Context,nodeID,key,filename string,expiry time.Duration)(string,error){
 c,b,err:=d.client(ctx,nodeID);if err!=nil{return "",err};disposition:=mime.FormatMediaType("attachment",map[string]string{"filename":filename});out,err:=s3.NewPresignClient(c).PresignGetObject(ctx,&s3.GetObjectInput{Bucket:&b,Key:&key,ResponseContentDisposition:&disposition},s3.WithPresignExpires(expiry));if err!=nil{return "",err};return out.URL,nil
}
