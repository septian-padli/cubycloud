package mystorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type TrashItem struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Kind                  string    `json:"kind"`
	SizeBytes             int64     `json:"size_bytes"`
	ParentID              *string   `json:"parent_id"`
	ModifiedAt            time.Time `json:"modified_at"`
	DeletedAt             time.Time `json:"deleted_at"`
	DaysRemaining         int       `json:"days_remaining"`
	OriginalParentMissing bool      `json:"original_parent_missing"`
	DiskKey               *string   `json:"-"`
}

func (s *Service) ListTrash(ctx context.Context, owner string) ([]TrashItem, error) {
	query := `
		SELECT m.id, m.name, m.kind, m.size_bytes, m.parent_id, m.modified_at, m.deleted_at,
		       CASE
		           WHEN m.parent_id IS NULL THEN 0
		           WHEN p.id IS NULL OR p.deleted_at IS NOT NULL THEN 1
		           ELSE 0
		       END as original_parent_missing
		FROM my_storage_items m
		LEFT JOIN my_storage_items p ON m.parent_id = p.id AND p.owner_id = m.owner_id
		WHERE m.owner_id = ? AND m.deleted_at IS NOT NULL
		ORDER BY m.deleted_at DESC, m.name COLLATE NOCASE
	`
	rows, err := s.db.DB.QueryContext(ctx, query, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	items := make([]TrashItem, 0)
	for rows.Next() {
		var (
			t                     TrashItem
			deletedAt             time.Time
			originalParentMissing int
		)
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Kind,
			&t.SizeBytes,
			&t.ParentID,
			&t.ModifiedAt,
			&deletedAt,
			&originalParentMissing,
		); err != nil {
			return nil, err
		}
		t.DeletedAt = deletedAt
		elapsedDays := int(now.Sub(deletedAt).Hours() / 24)
		remaining := 30 - elapsedDays
		if remaining < 0 {
			remaining = 0
		}
		t.DaysRemaining = remaining
		t.OriginalParentMissing = (originalParentMissing == 1)
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Service) Restore(ctx context.Context, owner, id string, forceRoot bool) (Item, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, false, err
	}
	defer tx.Rollback()

	var item Item
	var deletedAt *time.Time
	err = tx.QueryRowContext(
		ctx,
		`SELECT id, name, kind, size_bytes, parent_id, modified_at, disk_key, deleted_at FROM my_storage_items WHERE id=? AND owner_id=?`,
		id, owner,
	).Scan(&item.ID, &item.Name, &item.Kind, &item.SizeBytes, &item.ParentID, &item.ModifiedAt, &item.DiskKey, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) || deletedAt == nil {
		return Item{}, false, errNotFound
	}
	if err != nil {
		return Item{}, false, err
	}

	restoredToRoot := false
	targetParent := item.ParentID
	if forceRoot || targetParent == nil {
		targetParent = nil
		if item.ParentID != nil {
			restoredToRoot = true
		}
	} else {
		var parentKind string
		var parentDeletedAt *time.Time
		err = tx.QueryRowContext(
			ctx,
			`SELECT kind, deleted_at FROM my_storage_items WHERE id=? AND owner_id=?`,
			*targetParent, owner,
		).Scan(&parentKind, &parentDeletedAt)
		if errors.Is(err, sql.ErrNoRows) || parentKind != "folder" || parentDeletedAt != nil {
			targetParent = nil
			restoredToRoot = true
		} else if err != nil {
			return Item{}, false, err
		}
	}

	resolvedName, err := uniqueName(ctx, tx, owner, targetParent, item.Name, item.ID)
	if err != nil {
		return Item{}, false, err
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`UPDATE my_storage_items SET name=?, parent_id=?, modified_at=?, deleted_at=NULL WHERE id=? AND owner_id=?`,
		resolvedName, targetParent, now, id, owner,
	)
	if err != nil {
		return Item{}, false, err
	}

	_, err = tx.ExecContext(
		ctx,
		`WITH RECURSIVE descendants(id) AS (
			SELECT id FROM my_storage_items WHERE parent_id=? AND owner_id=?
			UNION ALL
			SELECT m.id FROM my_storage_items m JOIN descendants d ON m.parent_id=d.id WHERE m.owner_id=?
		) UPDATE my_storage_items SET deleted_at=NULL WHERE id IN (SELECT id FROM descendants)`,
		id, owner, owner,
	)
	if err != nil {
		return Item{}, false, err
	}

	if err = tx.Commit(); err != nil {
		return Item{}, false, err
	}

	item.Name = resolvedName
	item.ParentID = targetParent
	item.ModifiedAt = now
	return item, restoredToRoot, nil
}

func (s *Service) PurgeItem(ctx context.Context, owner, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var deletedAt *time.Time
	err = tx.QueryRowContext(
		ctx,
		`SELECT deleted_at FROM my_storage_items WHERE id=? AND owner_id=?`,
		id, owner,
	).Scan(&deletedAt)
	if errors.Is(err, sql.ErrNoRows) || deletedAt == nil {
		return errNotFound
	}
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(
		ctx,
		`WITH RECURSIVE descendants(id, kind, disk_key, node_path) AS (
			SELECT id, kind, disk_key, COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM my_storage_items WHERE id=? AND owner_id=?
			UNION ALL
			SELECT m.id, m.kind, m.disk_key, COALESCE((SELECT path FROM storage_nodes WHERE id=m.storage_node_id), '') FROM my_storage_items m JOIN descendants d ON m.parent_id=d.id WHERE m.owner_id=?
		) SELECT id, kind, disk_key, node_path FROM descendants`,
		id, owner, owner,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type toPurge struct {
		id, kind, nodePath string
		diskKey *string
	}
	var items []toPurge
	var ids []any
	for rows.Next() {
		var p toPurge
		if err := rows.Scan(&p.id, &p.kind, &p.diskKey, &p.nodePath); err != nil {
			return err
		}
		items = append(items, p)
		ids = append(ids, p.id)
	}
	_ = rows.Close()

	if len(ids) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	_, _ = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM vfs_starred WHERE node_id IN (%s)`, placeholders), ids...)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM my_storage_items WHERE id IN (%s)`, placeholders), ids...)
	if err != nil {
		return err
	}

	type blobToDelete struct{ key, nodePath string }
	var diskKeysToDelete []blobToDelete
	for _, p := range items {
		if p.kind == "file" && p.diskKey != nil && *p.diskKey != "" {
			var count int
			err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM my_storage_items WHERE disk_key=?`, *p.diskKey).Scan(&count)
			if err != nil {
				return err
			}
			if count == 0 {
				diskKeysToDelete = append(diskKeysToDelete, blobToDelete{*p.diskKey, p.nodePath})
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	for _, blob := range diskKeysToDelete {
		_ = s.storage.DeleteLocal(ctx, blob.nodePath, blob.key)
	}

	return nil
}

func (s *Service) PurgeAll(ctx context.Context, owner string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(
		ctx,
		`SELECT disk_key, COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM my_storage_items WHERE owner_id=? AND deleted_at IS NOT NULL AND kind='file' AND disk_key IS NOT NULL`,
		owner,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type blobToDelete struct{ key, nodePath string }
	var diskKeys []blobToDelete
	for rows.Next() {
		var blob blobToDelete
		if err := rows.Scan(&blob.key, &blob.nodePath); err != nil {
			return err
		}
		diskKeys = append(diskKeys, blob)
	}
	_ = rows.Close()

	_, _ = tx.ExecContext(ctx, `DELETE FROM vfs_starred WHERE node_id IN (SELECT id FROM my_storage_items WHERE owner_id=? AND deleted_at IS NOT NULL)`, owner)
	_, err = tx.ExecContext(ctx, `DELETE FROM my_storage_items WHERE owner_id=? AND deleted_at IS NOT NULL`, owner)
	if err != nil {
		return err
	}

	var diskKeysToDelete []blobToDelete
	for _, blob := range diskKeys {
		var count int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM my_storage_items WHERE disk_key=?`, blob.key).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			diskKeysToDelete = append(diskKeysToDelete, blob)
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	for _, blob := range diskKeysToDelete {
		_ = s.storage.DeleteLocal(ctx, blob.nodePath, blob.key)
	}

	return nil
}

func (s *Service) PurgeExpired(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(
		ctx,
		`WITH RECURSIVE expired(id, kind, disk_key, node_path) AS (
			SELECT id, kind, disk_key, COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM my_storage_items WHERE deleted_at IS NOT NULL AND deleted_at <= ?
			UNION
			SELECT m.id, m.kind, m.disk_key, COALESCE((SELECT path FROM storage_nodes WHERE id=m.storage_node_id), '') FROM my_storage_items m JOIN expired e ON m.parent_id=e.id
		) SELECT id, kind, disk_key, node_path FROM expired`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type itemInfo struct {
		id, kind, nodePath string
		diskKey *string
	}
	var items []itemInfo
	var ids []any
	for rows.Next() {
		var info itemInfo
		if err := rows.Scan(&info.id, &info.kind, &info.diskKey, &info.nodePath); err != nil {
			return 0, err
		}
		items = append(items, info)
		ids = append(ids, info.id)
	}
	_ = rows.Close()

	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	_, _ = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM vfs_starred WHERE node_id IN (%s)`, placeholders), ids...)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM my_storage_items WHERE id IN (%s)`, placeholders), ids...)
	if err != nil {
		return 0, err
	}
	type blobToDelete struct{ key, nodePath string }
	var diskKeysToDelete []blobToDelete
	for _, item := range items {
		if item.kind == "file" && item.diskKey != nil && *item.diskKey != "" {
			var count int
			err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM my_storage_items WHERE disk_key=?`, *item.diskKey).Scan(&count)
			if err != nil {
				return 0, err
			}
			if count == 0 {
				diskKeysToDelete = append(diskKeysToDelete, blobToDelete{*item.diskKey, item.nodePath})
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	for _, blob := range diskKeysToDelete {
		_ = s.storage.DeleteLocal(ctx, blob.nodePath, blob.key)
	}

	return int64(len(ids)), nil
}
