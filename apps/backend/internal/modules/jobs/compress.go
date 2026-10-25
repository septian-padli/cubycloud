package jobs

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

type CompressHandler struct {
	db     *bun.DB
	driver storage.StorageDriver
}

func NewCompressHandler(db *bun.DB, driver storage.StorageDriver) *CompressHandler {
	return &CompressHandler{
		db:     db,
		driver: driver,
	}
}
type queryRunner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type zipEntryItem struct {
	id       string
	name     string
	kind     string
	size     int64
	parentID *string
	diskKey  *string
	nodeID   *string
}

func (h *CompressHandler) Execute(ctx context.Context, job *database.Job, updateProgress func(int)) (any, error) {
	var payload CompressZipPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	if len(payload.ItemIDs) == 0 {
		return nil, errors.New("no items provided to compress")
	}

	scope := payload.Scope
	if scope != "my_storage" && scope != "organization" {
		scope = "my_storage"
	}

	table := "my_storage_items"
	if scope == "organization" {
		table = "organization_items"
	}

	// 1. Verify TargetParentID exists if specified
	targetParentID := payload.TargetParentID
	if targetParentID != nil && *targetParentID != "" {
		var exists bool
		var err error
		if scope == "my_storage" {
			err = h.db.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM my_storage_items WHERE id=? AND owner_id=? AND kind='folder' AND deleted_at IS NULL)`,
				*targetParentID, payload.UserID,
			).Scan(&exists)
		} else {
			err = h.db.QueryRowContext(ctx,
				`SELECT EXISTS(SELECT 1 FROM organization_items WHERE id=? AND kind='folder' AND deleted_at IS NULL)`,
				*targetParentID,
			).Scan(&exists)
		}
		if err != nil {
			return nil, fmt.Errorf("verify target parent: %w", err)
		}
		if !exists {
			return nil, errors.New("target parent folder not found or deleted")
		}
	} else {
		targetParentID = nil
	}

	// 2. Collect items to compress
	var entries []zipFileToCompress
	var defaultArchiveName string
	var totalUncompressedBytes int64

	usedPaths := make(map[string]int)
	sanitizePath := func(p string) string {
		count := usedPaths[p]
		usedPaths[p]++
		if count == 0 {
			return p
		}
		ext := path.Ext(p)
		base := strings.TrimSuffix(p, ext)
		return fmt.Sprintf("%s (%d)%s", base, count, ext)
	}

	for _, itemID := range payload.ItemIDs {
		rootItem, err := h.fetchItem(ctx, table, scope, payload.UserID, itemID)
		if err != nil {
			return nil, fmt.Errorf("fetch item %s: %w", itemID, err)
		}

		if defaultArchiveName == "" {
			if len(payload.ItemIDs) == 1 {
				base := strings.TrimSuffix(rootItem.name, filepath.Ext(rootItem.name))
				defaultArchiveName = base + ".zip"
			} else {
				defaultArchiveName = "Archive.zip"
			}
		}

		if rootItem.kind == "file" {
			if rootItem.diskKey == nil {
				continue
			}
			entryPath := sanitizePath(rootItem.name)
			entries = append(entries, zipFileToCompress{
				relPath: entryPath,
				diskKey: *rootItem.diskKey,
				size:    rootItem.size,
			})
			totalUncompressedBytes += rootItem.size
		} else if rootItem.kind == "folder" {
			descendants, err := h.loadFolderDescendants(ctx, table, scope, payload.UserID, rootItem.id)
			if err != nil {
				return nil, fmt.Errorf("load folder %s descendants: %w", rootItem.name, err)
			}

			// Map item ID to Item for path reconstruction
			itemMap := make(map[string]zipEntryItem)
			for _, desc := range descendants {
				itemMap[desc.id] = desc
			}

			for _, desc := range descendants {
				if desc.kind != "file" || desc.diskKey == nil {
					continue
				}

				// Build relative path inside folder
				rel := desc.name
				currParent := desc.parentID
				for currParent != nil && *currParent != rootItem.id {
					if parentItem, ok := itemMap[*currParent]; ok {
						rel = path.Join(parentItem.name, rel)
						currParent = parentItem.parentID
					} else {
						break
					}
				}
				fullRel := path.Join(rootItem.name, rel)
				entryPath := sanitizePath(fullRel)

				entries = append(entries, zipFileToCompress{
					relPath: entryPath,
					diskKey: *desc.diskKey,
					size:    desc.size,
				})
				totalUncompressedBytes += desc.size
			}
		}
	}

	if len(entries) == 0 {
		return nil, errors.New("no files found to compress")
	}

	// 3. User Quota Pre-check (for My Storage)
	if scope == "my_storage" {
		if err := h.checkUserQuota(ctx, payload.UserID, totalUncompressedBytes); err != nil {
			return nil, err
		}
	}

	// 4. Stream files into temporary ZIP archive
	tempFile, err := os.CreateTemp("", fmt.Sprintf("cuby-zip-%s-*.zip", job.ID))
	if err != nil {
		return nil, fmt.Errorf("create temp zip: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	zw := zip.NewWriter(tempFile)
	var processedBytes int64

	for idx, entry := range entries {
		r, err := storage.ReadBlob(ctx, h.db, h.driver, table, entry.diskKey)
		if err != nil {
			_ = zw.Close()
			_ = tempFile.Close()
			return nil, fmt.Errorf("read blob %s for %s: %w", entry.diskKey, entry.relPath, err)
		}

		w, err := zw.Create(entry.relPath)
		if err != nil {
			r.Close()
			_ = zw.Close()
			_ = tempFile.Close()
			return nil, fmt.Errorf("create zip entry %s: %w", entry.relPath, err)
		}

		written, err := io.Copy(w, r)
		r.Close()
		if err != nil {
			_ = zw.Close()
			_ = tempFile.Close()
			return nil, fmt.Errorf("write zip entry %s: %w", entry.relPath, err)
		}

		processedBytes += written
		if updateProgress != nil && totalUncompressedBytes > 0 {
			pct := int(float64(processedBytes) / float64(totalUncompressedBytes) * 85)
			if pct > 85 {
				pct = 85
			}
			updateProgress(pct)
		} else if updateProgress != nil {
			pct := int(float64(idx+1) / float64(len(entries)) * 85)
			updateProgress(pct)
		}
	}

	if err := zw.Close(); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("close zip writer: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// 5. Measure actual archive size
	stat, err := os.Stat(tempPath)
	if err != nil {
		return nil, fmt.Errorf("stat temp zip: %w", err)
	}
	zipSize := stat.Size()

	if updateProgress != nil {
		updateProgress(90)
	}

	// 6. Placement & Node Capacity check
	placement, err := storage.SelectPlacement(ctx, h.db, h.driver, zipSize)
	if err != nil {
		return nil, fmt.Errorf("select storage node: %w", err)
	}

	// 7. Store archive blob physically to destination node
	diskKey := ulid.Make().String()
	readZip, err := os.Open(tempPath)
	if err != nil {
		return nil, fmt.Errorf("open temp zip: %w", err)
	}
	defer readZip.Close()

	if placement.Type == "LOCAL" {
		if err := h.driver.WriteLocal(ctx, placement.Path, diskKey, readZip); err != nil {
			return nil, fmt.Errorf("write local zip blob: %w", err)
		}
	} else if placement.Type == "S3" {
		cloudDriver, ok := h.driver.(storage.CloudStorageDriver)
		if !ok {
			return nil, errors.New("cloud driver not configured for S3 node")
		}
		if err := cloudDriver.WriteObject(ctx, placement.ID, diskKey, readZip, zipSize); err != nil {
			return nil, fmt.Errorf("write s3 zip blob: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported storage node type: %s", placement.Type)
	}

	// 8. Resolve Conflict-Safe Filename in Target Directory
	desiredName := payload.TargetName
	if desiredName == "" {
		desiredName = defaultArchiveName
	}
	if !strings.HasSuffix(strings.ToLower(desiredName), ".zip") {
		desiredName += ".zip"
	}

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.rollbackBlob(ctx, placement, diskKey)
		return nil, fmt.Errorf("begin db tx: %w", err)
	}
	defer tx.Rollback()

	finalName, err := h.resolveUniqueName(ctx, tx, table, scope, payload.UserID, targetParentID, desiredName)
	if err != nil {
		h.rollbackBlob(ctx, placement, diskKey)
		return nil, fmt.Errorf("resolve unique filename: %w", err)
	}

	// Verify quota in transaction if my_storage
	if scope == "my_storage" {
		if err := h.checkUserQuotaTx(ctx, tx, payload.UserID, zipSize); err != nil {
			h.rollbackBlob(ctx, placement, diskKey)
			return nil, err
		}
	}

	// 9. Commit VFS item metadata
	newItemID := ulid.Make().String()
	now := time.Now().UTC()

	if scope == "my_storage" {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
			 VALUES (?, ?, ?, 'file', ?, ?, ?, ?, ?)`,
			newItemID, payload.UserID, finalName, zipSize, targetParentID, diskKey, placement.ID, now,
		)
		if err != nil {
			h.rollbackBlob(ctx, placement, diskKey)
			return nil, fmt.Errorf("insert my_storage item: %w", err)
		}
		_ = database.RecordActivity(ctx, tx, payload.UserID, "create", newItemID, finalName, "file", "my-storage")
	} else {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
			 VALUES (?, ?, ?, 'file', ?, ?, ?, ?, ?)`,
			newItemID, payload.UserID, finalName, zipSize, targetParentID, diskKey, placement.ID, now,
		)
		if err != nil {
			h.rollbackBlob(ctx, placement, diskKey)
			return nil, fmt.Errorf("insert organization item: %w", err)
		}
		_ = database.RecordActivity(ctx, tx, payload.UserID, "create", newItemID, finalName, "file", "organization")
	}

	if err := tx.Commit(); err != nil {
		h.rollbackBlob(ctx, placement, diskKey)
		return nil, fmt.Errorf("commit metadata: %w", err)
	}

	if updateProgress != nil {
		updateProgress(100)
	}

	return CompressZipResult{
		ItemID:    newItemID,
		Name:      finalName,
		ParentID:  targetParentID,
		SizeBytes: zipSize,
		Scope:     scope,
	}, nil
}

type zipFileToCompress struct {
	relPath string
	diskKey string
	size    int64
}

func (h *CompressHandler) rollbackBlob(ctx context.Context, placement storage.Placement, diskKey string) {
	if placement.Type == "LOCAL" {
		_ = h.driver.DeleteLocal(ctx, placement.Path, diskKey)
	} else if placement.Type == "S3" {
		if cloudDriver, ok := h.driver.(storage.CloudStorageDriver); ok {
			_ = cloudDriver.DeleteObject(ctx, placement.ID, diskKey)
		}
	}
}

func (h *CompressHandler) fetchItem(ctx context.Context, table, scope, userID, itemID string) (zipEntryItem, error) {
	var item zipEntryItem
	var nodeID sql.NullString
	var diskKey sql.NullString
	var parentID sql.NullString

	var query string
	var row *sql.Row
	if scope == "my_storage" {
		query = fmt.Sprintf(`SELECT id, name, kind, size_bytes, parent_id, disk_key, storage_node_id FROM %s WHERE id=? AND owner_id=? AND deleted_at IS NULL`, table)
		row = h.db.QueryRowContext(ctx, query, itemID, userID)
	} else {
		query = fmt.Sprintf(`SELECT id, name, kind, size_bytes, parent_id, disk_key, storage_node_id FROM %s WHERE id=? AND deleted_at IS NULL`, table)
		row = h.db.QueryRowContext(ctx, query, itemID)
	}

	err := row.Scan(&item.id, &item.name, &item.kind, &item.size, &parentID, &diskKey, &nodeID)
	if err != nil {
		return item, err
	}
	if parentID.Valid {
		item.parentID = &parentID.String
	}
	if diskKey.Valid {
		item.diskKey = &diskKey.String
	}
	if nodeID.Valid {
		item.nodeID = &nodeID.String
	}
	return item, nil
}

func (h *CompressHandler) loadFolderDescendants(ctx context.Context, table, scope, userID, folderID string) ([]zipEntryItem, error) {
	var query string
	var rows *sql.Rows
	var err error

	if scope == "my_storage" {
		query = `
			WITH RECURSIVE tree(id, name, kind, size_bytes, parent_id, disk_key, storage_node_id) AS (
				SELECT id, name, kind, size_bytes, parent_id, disk_key, storage_node_id
				FROM my_storage_items
				WHERE parent_id=? AND owner_id=? AND deleted_at IS NULL
				UNION ALL
				SELECT m.id, m.name, m.kind, m.size_bytes, m.parent_id, m.disk_key, m.storage_node_id
				FROM my_storage_items m
				JOIN tree t ON m.parent_id=t.id
				WHERE m.owner_id=? AND m.deleted_at IS NULL
			)
			SELECT id, name, kind, size_bytes, parent_id, disk_key, storage_node_id FROM tree
		`
		rows, err = h.db.QueryContext(ctx, query, folderID, userID, userID)
	} else {
		query = `
			WITH RECURSIVE tree(id, name, kind, size_bytes, parent_id, disk_key, storage_node_id) AS (
				SELECT id, name, kind, size_bytes, parent_id, disk_key, storage_node_id
				FROM organization_items
				WHERE parent_id=? AND deleted_at IS NULL
				UNION ALL
				SELECT m.id, m.name, m.kind, m.size_bytes, m.parent_id, m.disk_key, m.storage_node_id
				FROM organization_items m
				JOIN tree t ON m.parent_id=t.id
				WHERE m.deleted_at IS NULL
			)
			SELECT id, name, kind, size_bytes, parent_id, disk_key, storage_node_id FROM tree
		`
		rows, err = h.db.QueryContext(ctx, query, folderID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []zipEntryItem
	for rows.Next() {
		var item zipEntryItem
		var parentID, diskKey, nodeID sql.NullString
		if err := rows.Scan(&item.id, &item.name, &item.kind, &item.size, &parentID, &diskKey, &nodeID); err != nil {
			return nil, err
		}
		if parentID.Valid {
			item.parentID = &parentID.String
		}
		if diskKey.Valid {
			item.diskKey = &diskKey.String
		}
		if nodeID.Valid {
			item.nodeID = &nodeID.String
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (h *CompressHandler) checkUserQuota(ctx context.Context, userID string, requiredBytes int64) error {
	var quota int64
	var role string
	if err := h.db.QueryRowContext(ctx, `SELECT quota_bytes, role FROM users WHERE id=?`, userID).Scan(&quota, &role); err != nil {
		return err
	}
	if role == "SUPER_ADMIN" || quota < 0 {
		return nil
	}

	var used int64
	if err := h.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT MAX(size_bytes) AS size_bytes FROM my_storage_items WHERE owner_id=? AND kind='file' GROUP BY disk_key)`,
		userID,
	).Scan(&used); err != nil {
		return err
	}

	if used+requiredBytes > quota {
		return errors.New("USER_QUOTA_EXCEEDED")
	}
	return nil
}

func (h *CompressHandler) checkUserQuotaTx(ctx context.Context, tx queryRunner, userID string, requiredBytes int64) error {
	var quota int64
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT quota_bytes, role FROM users WHERE id=?`, userID).Scan(&quota, &role); err != nil {
		return err
	}
	if role == "SUPER_ADMIN" || quota < 0 {
		return nil
	}

	var used int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT MAX(size_bytes) AS size_bytes FROM my_storage_items WHERE owner_id=? AND kind='file' GROUP BY disk_key)`,
		userID,
	).Scan(&used); err != nil {
		return err
	}

	if used+requiredBytes > quota {
		return errors.New("USER_QUOTA_EXCEEDED")
	}
	return nil
}

func (h *CompressHandler) resolveUniqueName(ctx context.Context, tx queryRunner, table, scope, userID string, parent *string, name string) (string, error) {
	base, ext := name, ""
	if !strings.HasSuffix(name, ".") {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
	}

	for n := 0; ; n++ {
		candidate := name
		if n > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		var count int
		var err error
		if scope == "my_storage" {
			err = tx.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`, table),
				userID, candidate, parent,
			).Scan(&count)
		} else {
			err = tx.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`, table),
				candidate, parent,
			).Scan(&count)
		}
		if err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
}
