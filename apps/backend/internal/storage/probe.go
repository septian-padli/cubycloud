package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/uptrace/bun"
)

const nodeProbeTimeout = 3 * time.Second

// ErrUnavailable marks an I/O failure caused by an inaccessible storage node.
var ErrUnavailable = errors.New("storage unavailable")

// NodeProber probes one configured node without exposing its identity.
type NodeProber interface {
	ProbeNode(context.Context, string) error
}

func (d *nodeDriver) ProbeNode(ctx context.Context, nodeID string) error {
	ctx, cancel := context.WithTimeout(ctx, nodeProbeTimeout)
	defer cancel()
	var nodeType string
	var nodePath sql.NullString
	if err := d.db.QueryRowContext(ctx, `SELECT type,path FROM storage_nodes WHERE id=?`, nodeID).Scan(&nodeType, &nodePath); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if nodeType == "LOCAL" {
		path := ""
		if nodePath.Valid {
			path = nodePath.String
		}
		_, _, err := d.StatLocal(ctx, path)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return nil
	}
	if nodeType != "S3" {
		return ErrUnavailable
	}
	client, bucket, err := d.client(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if _, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket}); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

// NodeAvailable reports node state while treating drivers without probe support
// as available for compatibility with in-process test and alternate drivers.
func NodeAvailable(ctx context.Context, driver StorageDriver, nodeID string) bool {
	if nodeID == "" {
		return true
	}
	prober, ok := driver.(NodeProber)
	if !ok {
		return true
	}
	probeCtx, cancel := context.WithTimeout(ctx, nodeProbeTimeout)
	defer cancel()
	return prober.ProbeNode(probeCtx, nodeID) == nil
}

// AllConfiguredNodesAvailable probes every registered storage node, including
// disabled nodes, and returns only a generic result to preserve node privacy.
func AllConfiguredNodesAvailable(ctx context.Context, db *bun.DB, driver StorageDriver) bool {
	rows, err := db.QueryContext(ctx, `SELECT id FROM storage_nodes`)
	if err != nil {
		return false
	}
	var nodeIDs []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil {
			_ = rows.Close()
			return false
		}
		nodeIDs = append(nodeIDs, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return false
	}
	if err := rows.Close(); err != nil {
		return false
	}
	available := true
	for _, id := range nodeIDs {
		if !NodeAvailable(ctx, driver, id) {
			available = false
		}
	}
	return available
}

