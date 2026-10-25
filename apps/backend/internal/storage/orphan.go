package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

type OrphanReconcileReport struct {
	LocalScanned int
	LocalDeleted int
	S3Scanned    int
	S3Deleted    int
	Errors       []string
}

type orphanNode struct {
	ID, Type, Path, Bucket string
}

func ReconcileOrphanBlobs(ctx context.Context, db *bun.DB, store StorageDriver, gracePeriod time.Duration) (*OrphanReconcileReport, error) {
	report := &OrphanReconcileReport{}
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Minute
	}
	var nodes []orphanNode
	rows, err := db.QueryContext(ctx, `SELECT id, type, COALESCE(path,''), COALESCE(bucket,'') FROM storage_nodes`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var node orphanNode
		if err := rows.Scan(&node.ID, &node.Type, &node.Path, &node.Bucket); err != nil {
			rows.Close()
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	referenced := make(map[string]struct{})
	for _, query := range []string{
		`SELECT disk_key FROM my_storage_items WHERE kind='file' AND disk_key IS NOT NULL AND disk_key != ''`,
		`SELECT disk_key FROM organization_items WHERE kind='file' AND disk_key IS NOT NULL AND disk_key != ''`,
		`SELECT id FROM pending_cloud_uploads WHERE expires_at > CURRENT_TIMESTAMP`,
		`SELECT s3_key FROM multipart_uploads WHERE (status='active' OR status='completed') AND expires_at > CURRENT_TIMESTAMP`,
	} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				rows.Close()
				return nil, err
			}
			referenced[key] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	for _, node := range nodes {
		switch strings.ToUpper(node.Type) {
		case "LOCAL":
			nodePath := node.Path
			if nodePath == "" {
				if driver, ok := store.(*nodeDriver); ok {
					nodePath = driver.basePath
				}
			}
			entries, err := os.ReadDir(nodePath)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("scan local node %s: %v", node.ID, err))
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("stat local blob %s/%s: %v", node.ID, entry.Name(), err))
					continue
				}
				if time.Since(info.ModTime()) < gracePeriod {
					continue
				}
				report.LocalScanned++
				if _, ok := referenced[entry.Name()]; ok {
					continue
				}
				if err := store.DeleteLocal(ctx, nodePath, entry.Name()); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("delete local blob %s/%s: %v", node.ID, entry.Name(), err))
					continue
				}
				report.LocalDeleted++
			}
		case "S3":
			cloud, ok := store.(CloudStorageDriver)
			if !ok {
				report.Errors = append(report.Errors, fmt.Sprintf("scan S3 node %s: cloud storage unsupported", node.ID))
				continue
			}
			keys, err := cloud.ListObjects(ctx, node.ID, "")
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("scan S3 node %s: %v", node.ID, err))
				continue
			}
			for _, key := range keys {
				if key == "" || strings.HasPrefix(key, ".") {
					continue
				}
				report.S3Scanned++
				if _, ok := referenced[key]; ok {
					continue
				}
				if err := cloud.DeleteObject(ctx, node.ID, key); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("delete S3 blob %s/%s: %v", node.ID, key, err))
					continue
				}
				report.S3Deleted++
			}
		}
	}
	return report, nil
}

