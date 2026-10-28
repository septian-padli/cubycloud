package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

const defaultOrphanGracePeriod = 72 * time.Hour

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

// ReconcileOrphanBlobs audits by default. Pass false explicitly to enable deletion.
func ReconcileOrphanBlobs(ctx context.Context, db *bun.DB, store StorageDriver, gracePeriod time.Duration, dryRun ...bool) (*OrphanReconcileReport, error) {
	deleteEnabled := len(dryRun) > 0 && !dryRun[0]
	if len(dryRun) > 1 {
		return nil, errors.New("specify at most one dry-run mode")
	}
	if gracePeriod <= 0 {
		gracePeriod = defaultOrphanGracePeriod
	}
	instanceID, err := InstanceID(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("load storage instance ID: %w", err)
	}
	report := &OrphanReconcileReport{}
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
	if err := rows.Close(); err != nil {
		return nil, err
	}

	// A queued migration or compression job can create or move blobs after the snapshot.
	var activeJobs int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status IN ('PENDING','PROCESSING') AND type IN ('node_migration','compress_zip')`).Scan(&activeJobs); err != nil {
		return nil, fmt.Errorf("check active storage jobs: %w", err)
	}
	if activeJobs != 0 {
		return nil, errors.New("orphan reconciliation aborted: active storage job")
	}

	referenced := make(map[string]struct{})
	rows, err = db.QueryContext(ctx, `SELECT disk_key FROM my_storage_items WHERE kind='file' AND disk_key IS NOT NULL AND disk_key != ''`)
	if err != nil {
		return nil, fmt.Errorf("load my storage references: %w", err)
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rows, err = db.QueryContext(ctx, `SELECT disk_key FROM organization_items WHERE kind='file' AND disk_key IS NOT NULL AND disk_key != ''`)
	if err != nil {
		return nil, fmt.Errorf("load organization references: %w", err)
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rows, err = db.QueryContext(ctx, `SELECT storage_node_id,id FROM pending_cloud_uploads WHERE expires_at > CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, fmt.Errorf("load pending upload references: %w", err)
	}
	for rows.Next() {
		var nodeID, id string
		if err := rows.Scan(&nodeID, &id); err != nil {
			rows.Close()
			return nil, err
		}
		referenced[BuildBlobKey(instanceID, nodeID, id)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rows, err = db.QueryContext(ctx, `SELECT s3_key FROM multipart_uploads WHERE status IN ('active','completed') AND expires_at > CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, fmt.Errorf("load multipart references: %w", err)
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if node.ID == "" || node.ID == "." || node.ID == ".." || strings.ContainsAny(node.ID, `/\\`) {
			return nil, fmt.Errorf("storage node %q is not a safe namespace component", node.ID)
		}
		prefix := fmt.Sprintf("cubycloud/%s/%s/blobs/", instanceID, node.ID)
		switch strings.ToUpper(node.Type) {
		case "LOCAL":
			basePath := node.Path
			if basePath == "" {
				if driver, ok := store.(*nodeDriver); ok {
					basePath = driver.basePath
				} else {
					return nil, fmt.Errorf("local storage node %s has no path", node.ID)
				}
			}
			root := filepath.Join(basePath, filepath.FromSlash(prefix))
			if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if path == root || entry.IsDir() {
					return nil
				}
				if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
					return nil
				}
				rel, err := filepath.Rel(root, path)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
					return fmt.Errorf("local blob escaped namespace directory")
				}
				key := prefix + filepath.ToSlash(rel)
				if !strings.HasPrefix(key, prefix) {
					return fmt.Errorf("local blob outside owned namespace")
				}
				if _, err := entry.Info(); err != nil {
					return err
				}
				report.LocalScanned++
				if _, ok := referenced[key]; ok {
					_, err := db.ExecContext(ctx, `DELETE FROM orphan_candidates WHERE storage_node_id=? AND disk_key=?`, node.ID, key)
					return err
				}
				eligible, err := observeOrphan(ctx, db, node.ID, key, time.Now().UTC(), gracePeriod)
				if err != nil {
					return err
				}
				if !eligible || !deleteEnabled {
					return nil
				}
				if err := store.DeleteLocal(ctx, basePath, key); err != nil {
					return err
				}
				report.LocalDeleted++
				_, err = db.ExecContext(ctx, `DELETE FROM orphan_candidates WHERE storage_node_id=? AND disk_key=?`, node.ID, key)
				return err
			}); err != nil {
				return nil, fmt.Errorf("scan local storage node %s: %w", node.ID, err)
			}
		case "S3":
			cloud, ok := store.(CloudStorageDriver)
			if !ok {
				return nil, fmt.Errorf("scan S3 node %s: cloud storage unsupported", node.ID)
			}
			keys, err := cloud.ListObjects(ctx, node.ID, prefix)
			if err != nil {
				return nil, fmt.Errorf("scan S3 node %s: %w", node.ID, err)
			}
			for _, key := range keys {
				if !strings.HasPrefix(key, prefix) {
					return nil, fmt.Errorf("S3 returned object outside owned namespace for node %s", node.ID)
				}
				keyInstance, keyNode, _, ok := ParseBlobKey(key)
				if !ok || keyInstance != instanceID || keyNode != node.ID {
					return nil, fmt.Errorf("invalid namespaced S3 key %q", key)
				}
				report.S3Scanned++
				if _, ok := referenced[key]; ok {
					continue
				}
				eligible, err := observeOrphan(ctx, db, node.ID, key, time.Now().UTC(), gracePeriod)
				if err != nil {
					return nil, err
				}
				if !eligible || !deleteEnabled {
					continue
				}
				if err := cloud.DeleteObject(ctx, node.ID, key); err != nil {
					return nil, fmt.Errorf("delete S3 blob %s: %w", key, err)
				}
				report.S3Deleted++
				if _, err := db.ExecContext(ctx, `DELETE FROM orphan_candidates WHERE storage_node_id=? AND disk_key=?`, node.ID, key); err != nil {
					return nil, err
				}
			}
		}
	}
	return report, nil
}

func observeOrphan(ctx context.Context, db *bun.DB, nodeID, key string, now time.Time, grace time.Duration) (bool, error) {
	instanceID, parsedNodeID, _, ok := ParseBlobKey(key)
	if !ok || instanceID == "" || parsedNodeID != nodeID {
		return false, fmt.Errorf("refusing out-of-namespace orphan candidate %q", key)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO orphan_candidates(storage_node_id,disk_key,first_seen_at,last_seen_at,observation_count) VALUES(?,?,?, ?,1) ON CONFLICT(storage_node_id,disk_key) DO UPDATE SET last_seen_at=excluded.last_seen_at, observation_count=orphan_candidates.observation_count+1`, nodeID, key, now, now); err != nil {
		return false, err
	}
	var firstSeen, lastSeen time.Time
	var observations int
	if err := db.QueryRowContext(ctx, `SELECT first_seen_at,last_seen_at,observation_count FROM orphan_candidates WHERE storage_node_id=? AND disk_key=?`, nodeID, key).Scan(&firstSeen, &lastSeen, &observations); err != nil {
		return false, err
	}
	return observations >= 2 && lastSeen.After(firstSeen) && now.Sub(firstSeen) >= grace, nil
}

