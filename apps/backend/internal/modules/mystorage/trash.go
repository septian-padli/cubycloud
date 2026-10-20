package mystorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cubycloud/backend/internal/storage"
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

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func countDiskKeyRefs(ctx context.Context, q rowQuerier, diskKey string) (int, error) {
	var c1, c2 int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM my_storage_items WHERE disk_key=?`, diskKey).Scan(&c1); err != nil {
		return 0, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_items WHERE disk_key=?`, diskKey).Scan(&c2); err != nil {
		return 0, err
	}
	return c1 + c2, nil
}

func orgUniqueName(
	ctx context.Context,
	q interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	parent *string,
	name, exclude string,
) (string, error) {
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
		err := q.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') AND id<>?`,
			candidate,
			parent,
			exclude,
		).Scan(&count)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
}

func (s *Service) ListTrash(ctx context.Context, owner string) ([]TrashItem, error) {
	query := `
		SELECT id, name, kind, size_bytes, parent_id, modified_at, deleted_at, original_parent_missing FROM (
			SELECT m.id, m.name, m.kind, m.size_bytes, m.parent_id, m.modified_at, m.deleted_at,
			       CASE
			           WHEN m.parent_id IS NULL THEN 0
			           WHEN p.id IS NULL OR p.deleted_at IS NOT NULL THEN 1
			           ELSE 0
			       END as original_parent_missing
			FROM my_storage_items m
			LEFT JOIN my_storage_items p ON m.parent_id = p.id AND p.owner_id = m.owner_id
			WHERE m.owner_id = ? AND m.deleted_at IS NOT NULL

			UNION ALL

			SELECT o.id, o.name, o.kind, o.size_bytes, o.parent_id, o.modified_at, o.deleted_at,
			       CASE
			           WHEN o.parent_id IS NULL THEN 0
			           WHEN p.id IS NULL OR p.deleted_at IS NOT NULL THEN 1
			           ELSE 0
			       END as original_parent_missing
			FROM organization_items o
			LEFT JOIN organization_items p ON o.parent_id = p.id
			WHERE o.deleted_by = ? AND o.deleted_at IS NOT NULL
		)
		ORDER BY deleted_at DESC, name COLLATE NOCASE
	`
	rows, err := s.db.DB.QueryContext(ctx, query, owner, owner)
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

	isOrg := false
	if errors.Is(err, sql.ErrNoRows) || deletedAt == nil {
		err = tx.QueryRowContext(
			ctx,
			`SELECT id, name, kind, size_bytes, parent_id, modified_at, disk_key, deleted_at FROM organization_items WHERE id=? AND deleted_by=?`,
			id, owner,
		).Scan(&item.ID, &item.Name, &item.Kind, &item.SizeBytes, &item.ParentID, &item.ModifiedAt, &item.DiskKey, &deletedAt)
		if errors.Is(err, sql.ErrNoRows) || deletedAt == nil {
			return Item{}, false, errNotFound
		}
		if err != nil {
			return Item{}, false, err
		}
		isOrg = true
	} else if err != nil {
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
		if isOrg {
			err = tx.QueryRowContext(
				ctx,
				`SELECT kind, deleted_at FROM organization_items WHERE id=?`,
				*targetParent,
			).Scan(&parentKind, &parentDeletedAt)
		} else {
			err = tx.QueryRowContext(
				ctx,
				`SELECT kind, deleted_at FROM my_storage_items WHERE id=? AND owner_id=?`,
				*targetParent, owner,
			).Scan(&parentKind, &parentDeletedAt)
		}
		if errors.Is(err, sql.ErrNoRows) || parentKind != "folder" || parentDeletedAt != nil {
			targetParent = nil
			restoredToRoot = true
		} else if err != nil {
			return Item{}, false, err
		}
	}

	var resolvedName string
	if isOrg {
		resolvedName, err = orgUniqueName(ctx, tx, targetParent, item.Name, item.ID)
	} else {
		resolvedName, err = uniqueName(ctx, tx, owner, targetParent, item.Name, item.ID)
	}
	if err != nil {
		return Item{}, false, err
	}

	now := time.Now().UTC()
	if isOrg {
		_, err = tx.ExecContext(
			ctx,
			`UPDATE organization_items SET name=?, parent_id=?, modified_at=?, deleted_at=NULL, deleted_by=NULL WHERE id=?`,
			resolvedName, targetParent, now, id,
		)
		if err != nil {
			return Item{}, false, err
		}

		_, err = tx.ExecContext(
			ctx,
			`WITH RECURSIVE descendants(id) AS (
				SELECT id FROM organization_items WHERE parent_id=?
				UNION ALL
				SELECT m.id FROM organization_items m JOIN descendants d ON m.parent_id=d.id
			) UPDATE organization_items SET deleted_at=NULL, deleted_by=NULL WHERE id IN (SELECT id FROM descendants)`,
			id,
		)
		if err != nil {
			return Item{}, false, err
		}
	} else {
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

	isOrg := false
	if errors.Is(err, sql.ErrNoRows) || deletedAt == nil {
		err = tx.QueryRowContext(
			ctx,
			`SELECT deleted_at FROM organization_items WHERE id=? AND deleted_by=?`,
			id, owner,
		).Scan(&deletedAt)
		if errors.Is(err, sql.ErrNoRows) || deletedAt == nil {
			return errNotFound
		}
		if err != nil {
			return err
		}
		isOrg = true
	} else if err != nil {
		return err
	}

	type toPurge struct {
		id, kind, nodeID, nodePath string
		diskKey                    *string
	}
	var items []toPurge
	var ids []any

	if isOrg {
		rows, err := tx.QueryContext(
			ctx,
			`WITH RECURSIVE descendants(id, kind, disk_key, node_id, node_path) AS (
				SELECT id, kind, disk_key, COALESCE(storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM organization_items WHERE id=?
				UNION ALL
				SELECT m.id, m.kind, m.disk_key, COALESCE(m.storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=m.storage_node_id), '') FROM organization_items m JOIN descendants d ON m.parent_id=d.id
			) SELECT id, kind, disk_key, node_id, node_path FROM descendants`,
			id,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p toPurge
			if err := rows.Scan(&p.id, &p.kind, &p.diskKey, &p.nodeID, &p.nodePath); err != nil {
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
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM organization_items WHERE id IN (%s)`, placeholders), ids...)
		if err != nil {
			return err
		}
	} else {
		rows, err := tx.QueryContext(
			ctx,
			`WITH RECURSIVE descendants(id, kind, disk_key, node_id, node_path) AS (
				SELECT id, kind, disk_key, COALESCE(storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM my_storage_items WHERE id=? AND owner_id=?
				UNION ALL
				SELECT m.id, m.kind, m.disk_key, COALESCE(m.storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=m.storage_node_id), '') FROM my_storage_items m JOIN descendants d ON m.parent_id=d.id WHERE m.owner_id=?
			) SELECT id, kind, disk_key, node_id, node_path FROM descendants`,
			id, owner, owner,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p toPurge
			if err := rows.Scan(&p.id, &p.kind, &p.diskKey, &p.nodeID, &p.nodePath); err != nil {
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
	}

	type blobToDelete struct{ key, nodeID, nodePath string }
	var diskKeysToDelete []blobToDelete
	seen := make(map[string]bool)
	for _, p := range items {
		if p.kind == "file" && p.diskKey != nil && *p.diskKey != "" && !seen[*p.diskKey] {
			seen[*p.diskKey] = true
			count, err := countDiskKeyRefs(ctx, tx, *p.diskKey)
			if err != nil {
				return err
			}
			if count == 0 {
				diskKeysToDelete = append(diskKeysToDelete, blobToDelete{*p.diskKey, p.nodeID, p.nodePath})
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	for _, blob := range diskKeysToDelete {
		_ = storage.DeleteBlob(ctx, s.db, s.storage, blob.nodeID, blob.nodePath, blob.key)
	}

	return nil
}

func (s *Service) PurgeAll(ctx context.Context, owner string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	type blobCandidate struct{ key, nodeID, nodePath string }
	var diskKeys []blobCandidate

	// Collect from my_storage_items
	rows, err := tx.QueryContext(
		ctx,
		`SELECT disk_key, COALESCE(storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM my_storage_items WHERE owner_id=? AND deleted_at IS NOT NULL AND kind='file' AND disk_key IS NOT NULL`,
		owner,
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var blob blobCandidate
		if err := rows.Scan(&blob.key, &blob.nodeID, &blob.nodePath); err != nil {
			rows.Close()
			return err
		}
		diskKeys = append(diskKeys, blob)
	}
	rows.Close()

	// Collect from organization_items
	rowsOrg, err := tx.QueryContext(
		ctx,
		`SELECT disk_key, COALESCE(storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM organization_items WHERE deleted_by=? AND deleted_at IS NOT NULL AND kind='file' AND disk_key IS NOT NULL`,
		owner,
	)
	if err != nil {
		return err
	}
	for rowsOrg.Next() {
		var blob blobCandidate
		if err := rowsOrg.Scan(&blob.key, &blob.nodeID, &blob.nodePath); err != nil {
			rowsOrg.Close()
			return err
		}
		diskKeys = append(diskKeys, blob)
	}
	rowsOrg.Close()

	_, _ = tx.ExecContext(ctx, `DELETE FROM vfs_starred WHERE node_id IN (SELECT id FROM my_storage_items WHERE owner_id=? AND deleted_at IS NOT NULL)`, owner)
	_, _ = tx.ExecContext(ctx, `DELETE FROM vfs_starred WHERE node_id IN (SELECT id FROM organization_items WHERE deleted_by=? AND deleted_at IS NOT NULL)`, owner)

	_, err = tx.ExecContext(ctx, `DELETE FROM my_storage_items WHERE owner_id=? AND deleted_at IS NOT NULL`, owner)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM organization_items WHERE deleted_by=? AND deleted_at IS NOT NULL`, owner)
	if err != nil {
		return err
	}

	var diskKeysToDelete []blobCandidate
	seen := make(map[string]bool)
	for _, blob := range diskKeys {
		if blob.key != "" && !seen[blob.key] {
			seen[blob.key] = true
			count, err := countDiskKeyRefs(ctx, tx, blob.key)
			if err != nil {
				return err
			}
			if count == 0 {
				diskKeysToDelete = append(diskKeysToDelete, blob)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	for _, blob := range diskKeysToDelete {
		_ = storage.DeleteBlob(ctx, s.db, s.storage, blob.nodeID, blob.nodePath, blob.key)
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

	type itemInfo struct {
		id, kind, nodeID, nodePath string
		diskKey                    *string
	}

	// 1. My Storage Expired Items
	rowsMy, err := tx.QueryContext(
		ctx,
		`WITH RECURSIVE expired(id, kind, disk_key, node_id, node_path) AS (
			SELECT id, kind, disk_key, COALESCE(storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM my_storage_items WHERE deleted_at IS NOT NULL AND deleted_at <= ?
			UNION
			SELECT m.id, m.kind, m.disk_key, COALESCE(m.storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=m.storage_node_id), '') FROM my_storage_items m JOIN expired e ON m.parent_id=e.id
		) SELECT id, kind, disk_key, node_id, node_path FROM expired`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	var myItems []itemInfo
	var myIDs []any
	for rowsMy.Next() {
		var info itemInfo
		if err := rowsMy.Scan(&info.id, &info.kind, &info.diskKey, &info.nodeID, &info.nodePath); err != nil {
			rowsMy.Close()
			return 0, err
		}
		myItems = append(myItems, info)
		myIDs = append(myIDs, info.id)
	}
	rowsMy.Close()

	// 2. Organization Expired Items
	rowsOrg, err := tx.QueryContext(
		ctx,
		`WITH RECURSIVE expired(id, kind, disk_key, node_id, node_path) AS (
			SELECT id, kind, disk_key, COALESCE(storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=storage_node_id), '') FROM organization_items WHERE deleted_at IS NOT NULL AND deleted_at <= ?
			UNION
			SELECT m.id, m.kind, m.disk_key, COALESCE(m.storage_node_id, ''), COALESCE((SELECT path FROM storage_nodes WHERE id=m.storage_node_id), '') FROM organization_items m JOIN expired e ON m.parent_id=e.id
		) SELECT id, kind, disk_key, node_id, node_path FROM expired`,
		cutoff,
	)
	if err != nil {
		return 0, err
	}
	var orgItems []itemInfo
	var orgIDs []any
	for rowsOrg.Next() {
		var info itemInfo
		if err := rowsOrg.Scan(&info.id, &info.kind, &info.diskKey, &info.nodeID, &info.nodePath); err != nil {
			rowsOrg.Close()
			return 0, err
		}
		orgItems = append(orgItems, info)
		orgIDs = append(orgIDs, info.id)
	}
	rowsOrg.Close()

	totalPurged := int64(len(myIDs) + len(orgIDs))
	if totalPurged == 0 {
		return 0, nil
	}

	if len(myIDs) > 0 {
		placeholders := strings.Repeat("?,", len(myIDs))
		placeholders = placeholders[:len(placeholders)-1]
		_, _ = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM vfs_starred WHERE node_id IN (%s)`, placeholders), myIDs...)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM my_storage_items WHERE id IN (%s)`, placeholders), myIDs...)
		if err != nil {
			return 0, err
		}
	}

	if len(orgIDs) > 0 {
		placeholders := strings.Repeat("?,", len(orgIDs))
		placeholders = placeholders[:len(placeholders)-1]
		_, _ = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM vfs_starred WHERE node_id IN (%s)`, placeholders), orgIDs...)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM organization_items WHERE id IN (%s)`, placeholders), orgIDs...)
		if err != nil {
			return 0, err
		}
	}

	type blobToDelete struct{ key, nodeID, nodePath string }
	var diskKeysToDelete []blobToDelete
	seen := make(map[string]bool)
	allItems := append(myItems, orgItems...)
	for _, item := range allItems {
		if item.kind == "file" && item.diskKey != nil && *item.diskKey != "" && !seen[*item.diskKey] {
			seen[*item.diskKey] = true
			count, err := countDiskKeyRefs(ctx, tx, *item.diskKey)
			if err != nil {
				return 0, err
			}
			if count == 0 {
				diskKeysToDelete = append(diskKeysToDelete, blobToDelete{*item.diskKey, item.nodeID, item.nodePath})
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, err
	}

	for _, blob := range diskKeysToDelete {
		_ = storage.DeleteBlob(ctx, s.db, s.storage, blob.nodeID, blob.nodePath, blob.key)
	}

	return totalPurged, nil
}
