package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/uptrace/bun"
)

type NodeMigrationHandler struct {
	db     *bun.DB
	driver storage.StorageDriver
}

func NewNodeMigrationHandler(db *bun.DB, driver storage.StorageDriver) *NodeMigrationHandler {
	return &NodeMigrationHandler{
		db:     db,
		driver: driver,
	}
}

type migrationBlob struct {
	DiskKey   string
	SizeBytes int64
}

func (h *NodeMigrationHandler) Execute(ctx context.Context, job *database.Job, updateProgress func(int)) (any, error) {
	var payload NodeMigrationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, fmt.Errorf("invalid migration payload: %w", err)
	}

	if payload.SourceNodeID == "" {
		return nil, errors.New("source_node_id is required")
	}

	var sourceNode database.StorageNode
	if err := h.db.NewSelect().Model(&sourceNode).Where("id = ?", payload.SourceNodeID).Scan(ctx); err != nil {
		return nil, fmt.Errorf("source node not found: %w", err)
	}

	// 1. Immediately disable source node if not already disabled to prevent any new allocations.
	if sourceNode.Enabled {
		_, err := h.db.NewUpdate().
			Model((*database.StorageNode)(nil)).
			Set("enabled = ?", false).
			Where("id = ?", sourceNode.ID).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to disable source node: %w", err)
		}
		sourceNode.Enabled = false
	}

	legacyNodeID := h.getLegacyNodeID(ctx)

	// 2. Query all distinct disk blobs on this source node across personal and organization tables.
	var blobs []migrationBlob
	rows, err := h.db.QueryContext(ctx, `
		SELECT disk_key, MAX(size_bytes) AS size_bytes
		FROM (
			SELECT disk_key, size_bytes
			FROM my_storage_items
			WHERE kind = 'file' AND disk_key IS NOT NULL AND disk_key != ''
				AND (storage_node_id = ? OR (storage_node_id IS NULL AND ? = ?))
			UNION ALL
			SELECT disk_key, size_bytes
			FROM organization_items
			WHERE kind = 'file' AND disk_key IS NOT NULL AND disk_key != ''
				AND (storage_node_id = ? OR (storage_node_id IS NULL AND ? = ?))
		)
		GROUP BY disk_key
	`, sourceNode.ID, sourceNode.ID, legacyNodeID, sourceNode.ID, sourceNode.ID, legacyNodeID)
	if err != nil {
		return nil, fmt.Errorf("query source blobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b migrationBlob
		if err := rows.Scan(&b.DiskKey, &b.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan blob row: %w", err)
		}
		blobs = append(blobs, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blob rows: %w", err)
	}

	total := len(blobs)
	if total == 0 {
		updateProgress(100)
		return &NodeMigrationResult{
			SourceNodeID:     sourceNode.ID,
			TargetNodeID:     payload.TargetNodeID,
			TotalFiles:       0,
			MigratedFiles:    0,
			FailedFiles:      0,
			SafeToDisconnect: true,
		}, nil
	}

	var predeterminedTarget *database.StorageNode
	if payload.TargetNodeID != "" && payload.TargetNodeID != sourceNode.ID {
		var target database.StorageNode
		if err := h.db.NewSelect().Model(&target).Where("id = ?", payload.TargetNodeID).Scan(ctx); err != nil {
			return nil, fmt.Errorf("target node not found: %w", err)
		}
		if !target.Enabled {
			return nil, errors.New("target node is disabled")
		}
		predeterminedTarget = &target
	}

	migrated := 0
	failed := 0

	for i, b := range blobs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		target := predeterminedTarget
		if target == nil {
			placement, err := storage.SelectPlacement(ctx, h.db, h.driver, b.SizeBytes)
			if err != nil {
				failed++
				continue
			}
			if placement.ID == sourceNode.ID {
				failed++
				continue
			}
			var t database.StorageNode
			if err := h.db.NewSelect().Model(&t).Where("id = ?", placement.ID).Scan(ctx); err != nil {
				failed++
				continue
			}
			target = &t
		}

		// Copy, verify hash/size, then update DB pointers. Never delete source files.
		if err := h.copyVerifyAndRehome(ctx, &sourceNode, target, b.DiskKey, b.SizeBytes, legacyNodeID); err != nil {
			failed++
		} else {
			migrated++
		}

		updateProgress(int(float64(i+1) / float64(total) * 100))
	}

	res := &NodeMigrationResult{
		SourceNodeID:     sourceNode.ID,
		TargetNodeID:     payload.TargetNodeID,
		TotalFiles:       total,
		MigratedFiles:    migrated,
		FailedFiles:      failed,
		SafeToDisconnect: failed == 0 && migrated == total,
	}

	if failed > 0 {
		res.ErrorMessage = fmt.Sprintf("migration completed with %d failed files; source files retained", failed)
		return res, fmt.Errorf("%s", res.ErrorMessage)
	}

	return res, nil
}

func (h *NodeMigrationHandler) copyVerifyAndRehome(
	ctx context.Context,
	sourceNode, targetNode *database.StorageNode,
	diskKey string,
	expectedSize int64,
	legacyNodeID string,
) error {
	// 1. Open source reader
	srcReader, err := h.openNodeReader(ctx, sourceNode, diskKey)
	if err != nil {
		return fmt.Errorf("open source blob: %w", err)
	}
	defer srcReader.Close()

	// 2. Stream to a temporary file while computing source SHA-256.
	// Provides a seekable stream (*os.File) compatible with S3/cloud drivers and bounded memory.
	tmpFile, err := os.CreateTemp("", "cubycloud-mig-*")
	if err != nil {
		return fmt.Errorf("create migration spool file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	srcHasher := sha256.New()
	mw := io.MultiWriter(tmpFile, srcHasher)
	spooledBytes, err := io.Copy(mw, srcReader)
	if err != nil {
		return fmt.Errorf("spool source blob: %w", err)
	}
	if expectedSize >= 0 && spooledBytes != expectedSize {
		return fmt.Errorf("source size mismatch: expected %d, got %d", expectedSize, spooledBytes)
	}
	srcDigest := hex.EncodeToString(srcHasher.Sum(nil))

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek spool file: %w", err)
	}

	// 3. Write to destination
	if err := h.writeNodeBlob(ctx, targetNode, diskKey, tmpFile, spooledBytes); err != nil {
		return fmt.Errorf("write destination blob: %w", err)
	}
	// 3. Read back from destination and verify SHA-256 and size
	dstReader, err := h.openNodeReader(ctx, targetNode, diskKey)
	if err != nil {
		return fmt.Errorf("read destination blob for verification: %w", err)
	}
	defer dstReader.Close()

	dstHasher := sha256.New()
	writtenBytes, err := io.Copy(dstHasher, dstReader)
	if err != nil {
		return fmt.Errorf("stream destination verification: %w", err)
	}
	dstDigest := hex.EncodeToString(dstHasher.Sum(nil))

	if expectedSize >= 0 && writtenBytes != expectedSize {
		return fmt.Errorf("verification size mismatch: expected %d, got %d", expectedSize, writtenBytes)
	}
	if srcDigest != dstDigest {
		return fmt.Errorf("verification checksum mismatch: expected %s, got %s", srcDigest, dstDigest)
	}

	// 4. Update database pointers inside an atomic transaction.
	// Zero deletion: source file is never removed.
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rehome transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE my_storage_items
		SET storage_node_id = ?
		WHERE disk_key = ? AND (storage_node_id = ? OR (storage_node_id IS NULL AND ? = ?))
	`, targetNode.ID, diskKey, sourceNode.ID, sourceNode.ID, legacyNodeID)
	if err != nil {
		return fmt.Errorf("update my_storage_items: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE organization_items
		SET storage_node_id = ?
		WHERE disk_key = ? AND (storage_node_id = ? OR (storage_node_id IS NULL AND ? = ?))
	`, targetNode.ID, diskKey, sourceNode.ID, sourceNode.ID, legacyNodeID)
	if err != nil {
		return fmt.Errorf("update organization_items: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rehome transaction: %w", err)
	}

	return nil
}

func (h *NodeMigrationHandler) openNodeReader(ctx context.Context, node *database.StorageNode, key string) (io.ReadCloser, error) {
	if node.Type == "S3" {
		cloudDriver, ok := h.driver.(storage.CloudStorageDriver)
		if !ok {
			return nil, errors.New("cloud storage driver unavailable")
		}
		return cloudDriver.ReadObject(ctx, node.ID, key)
	}

	nodePath := ""
	if node.Path != nil {
		nodePath = *node.Path
	}
	return h.driver.ReadLocal(ctx, nodePath, key)
}

func (h *NodeMigrationHandler) writeNodeBlob(ctx context.Context, node *database.StorageNode, key string, r io.Reader, size int64) error {
	if node.Type == "S3" {
		cloudDriver, ok := h.driver.(storage.CloudStorageDriver)
		if !ok {
			return errors.New("cloud storage driver unavailable")
		}
		return cloudDriver.WriteObject(ctx, node.ID, key, r, size)
	}

	nodePath := ""
	if node.Path != nil {
		nodePath = *node.Path
	}
	return h.driver.WriteLocal(ctx, nodePath, key, r)
}

func (h *NodeMigrationHandler) getLegacyNodeID(ctx context.Context) string {
	var nodes []database.StorageNode
	if err := h.db.NewSelect().Model(&nodes).Where("type = 'LOCAL'").Order("created_at ASC").Scan(ctx); err != nil || len(nodes) == 0 {
		return ""
	}

	for _, n := range nodes {
		if n.Path != nil && filepath.Base(*n.Path) == "default" {
			return n.ID
		}
	}
	return nodes[0].ID
}
