package mystorage

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

var (
	errNotFound    = errors.New("item not found")
	errConflict    = errors.New("name conflict")
	errQuota       = errors.New("USER_QUOTA_EXCEEDED")
	errStorageFull = errors.New("STORAGE_POOL_FULL")
	errInvalidName = errors.New("invalid name")
	errBadParent   = errors.New("parent is not a folder")
	errMoveCycle   = errors.New("cannot move folder into itself")

	ErrNotFound     = errNotFound
	ErrNameConflict = errors.New("name already in use")
	ErrCircularMove = errors.New("cannot move folder into itself or its descendant")
	ErrBadParent    = errBadParent
)

type Item struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	SizeBytes  int64     `json:"size_bytes"`
	ParentID   *string   `json:"parent_id"`
	ModifiedAt time.Time `json:"modified_at"`
	IsStarred  bool      `json:"is_starred"`
	DiskKey    *string   `json:"-"`
}

type VFSNode = Item

type Folder struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
}

type Service struct {
	db       *bun.DB
	storage  storage.StorageDriver
	uploadMu sync.Mutex
}

func NewService(db *bun.DB, driver storage.StorageDriver) *Service {
	return &Service{db: db, storage: driver}
}

func (s *Service) List(ctx context.Context, owner string, parent *string) ([]Item, []Folder, error) {
	query := `SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
		CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END AS is_starred
		FROM my_storage_items i
		LEFT JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.owner_id=? AND i.deleted_at IS NULL AND `
	args := []any{owner, owner}
	if parent == nil || *parent == "" {
		query += `i.parent_id IS NULL`
	} else {
		parentItem, err := s.get(ctx, owner, *parent)
		if err != nil {
			return nil, nil, err
		}
		if parentItem.Kind != "folder" {
			return nil, nil, errNotFound
		}
		query += `i.parent_id=?`
		args = append(args, *parent)
	}
	query += ` ORDER BY i.kind DESC, i.name COLLATE NOCASE`
	rows, err := s.db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var i Item
		if err = rows.Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.IsStarred); err != nil {
			return nil, nil, err
		}
		items = append(items, i)
	}
	return items, nil, rows.Err()
}

func (s *Service) Folders(ctx context.Context, owner string) ([]Folder, error) {
	rows, err := s.db.DB.QueryContext(
		ctx,
		`SELECT id,name,parent_id FROM my_storage_items WHERE owner_id=? AND kind='folder' AND deleted_at IS NULL ORDER BY name COLLATE NOCASE`,
		owner,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Folder, 0)
	for rows.Next() {
		var f Folder
		if err = rows.Scan(&f.ID, &f.Name, &f.ParentID); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) Breadcrumbs(ctx context.Context, owner string, parent *string) ([]Folder, error) {
	out := make([]Folder, 0)
	id := parent
	for id != nil && *id != "" {
		var f Folder
		err := s.db.DB.QueryRowContext(
			ctx,
			`SELECT id,name,parent_id FROM my_storage_items WHERE id=? AND owner_id=? AND kind='folder' AND deleted_at IS NULL`,
			*id,
			owner,
		).Scan(&f.ID, &f.Name, &f.ParentID)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
		id = f.ParentID
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *Service) get(ctx context.Context, owner, id string) (Item, error) {
	var i Item
	err := s.db.DB.QueryRowContext(
		ctx,
		`SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
			CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END AS is_starred
		FROM my_storage_items i
		LEFT JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.id=? AND i.owner_id=? AND i.deleted_at IS NULL`,
		owner,
		id,
		owner,
	).Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.IsStarred)
	if errors.Is(err, sql.ErrNoRows) {
		return i, errNotFound
	}
	return i, err
}

func uniqueName(
	ctx context.Context,
	q interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	owner string,
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
			`SELECT COUNT(*) FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') AND id<>?`,
			owner,
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

func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00") && len(name) <= 255
}

func (s *Service) CreateFolder(ctx context.Context, owner, name string, parent *string, unique bool) (Item, string, error) {
	parent = normalizeParent(parent)
	if !validName(name) {
		return Item{}, "", errInvalidName
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, "", err
	}
	defer tx.Rollback()
	if err = s.checkParent(ctx, tx, owner, parent); err != nil {
		return Item{}, "", err
	}
	if unique {
		name, err = uniqueName(ctx, tx, owner, parent, name, "")
		if err != nil {
			return Item{}, "", err
		}
	} else {
		var existingID, existingKind string
		err = tx.QueryRowContext(
			ctx,
			`SELECT id, kind FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') LIMIT 1`,
			owner,
			name,
			parent,
		).Scan(&existingID, &existingKind)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Item{}, "", err
		}
		if err == nil {
			suggestion, e := uniqueName(ctx, tx, owner, parent, name, "")
			if e != nil {
				return Item{}, "", e
			}
			return Item{ID: existingID, Kind: existingKind, Name: name, ParentID: parent}, suggestion, errConflict
		}
	}
	i := Item{
		ID:         ulid.Make().String(),
		Name:       name,
		Kind:       "folder",
		ParentID:   parent,
		ModifiedAt: time.Now().UTC(),
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO my_storage_items(id,owner_id,name,kind,parent_id,modified_at) VALUES(?,?,?,?,?,?)`,
		i.ID,
		owner,
		name,
		"folder",
		parent,
		i.ModifiedAt,
	)
	if err != nil {
		return Item{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return Item{}, "", err
	}
	return i, "", nil
}

func (s *Service) checkParent(ctx context.Context, tx quotaQuery, owner string, parent *string) error {
	if parent == nil || *parent == "" {
		return nil
	}
	var kind string
	err := tx.QueryRowContext(
		ctx,
		`SELECT kind FROM my_storage_items WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
		*parent,
		owner,
	).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	if kind != "folder" {
		return errBadParent
	}
	return nil
}

func normalizeParent(parent *string) *string {
	if parent == nil || *parent == "" {
		return nil
	}
	return parent
}

type countingReader struct {
	io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.n += int64(n)
	return n, err
}

func (s *Service) localPlacement(ctx context.Context) (string, string, int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(path,''), capacity_bytes, enabled FROM storage_nodes WHERE type='LOCAL' ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return "", "", 0, err
	}
	type localNode struct {
		id, path string
		capacity int64
		enabled  bool
	}
	var all []localNode
	for rows.Next() {
		var n localNode
		var enabled int
		if err := rows.Scan(&n.id, &n.path, &n.capacity, &enabled); err != nil {
			_ = rows.Close()
			return "", "", 0, err
		}
		n.enabled = enabled != 0
		all = append(all, n)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return "", "", 0, err
	}
	legacyID := ""
	for _, n := range all {
		if legacyID == "" {
			legacyID = n.id
		}
		if strings.HasSuffix(filepath.ToSlash(n.path), "/storage/default") {
			legacyID = n.id
			break
		}
	}
	var candidates []localNode
	for _, n := range all {
		if n.enabled {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return "", "", 0, nil
	}
	for i := range candidates {
		var used int64
		err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT disk_key,MAX(size_bytes) size_bytes FROM my_storage_items WHERE kind='file' AND disk_key IS NOT NULL AND (storage_node_id=? OR (storage_node_id IS NULL AND ?=?)) GROUP BY disk_key)`, candidates[i].id, candidates[i].id, legacyID).Scan(&used)
		if err != nil {
			return "", "", 0, err
		}
		reserve := candidates[i].capacity / 20
		if candidates[i].capacity%20 != 0 {
			reserve++
		}
		candidates[i].capacity -= used + reserve
		physicalCapacity, physicalFree, statErr := s.storage.StatLocal(ctx, candidates[i].path)
		if statErr != nil {
			return "", "", 0, statErr
		}
		if physicalCapacity > 0 {
			physicalReserve := physicalCapacity / 20
			if physicalCapacity%20 != 0 {
				physicalReserve++
			}
			physicalFree -= physicalReserve
			if physicalFree < candidates[i].capacity {
				candidates[i].capacity = physicalFree
			}
		}
	}
	var availableNodes []localNode
	for _, n := range candidates {
		if n.capacity > 0 {
			availableNodes = append(availableNodes, n)
		}
	}
	if len(availableNodes) == 0 {
		return candidates[0].id, candidates[0].path, 0, nil
	}
	candidates = availableNodes
	selected := 0
	var strategy string
	_ = s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='placement_strategy'`).Scan(&strategy)
	switch strategy {
	case "MOST_FREE_SPACE":
		for i := range candidates {
			if candidates[i].capacity > candidates[selected].capacity {
				selected = i
			}
		}
	case "ROUND_ROBIN":
		selected = int(time.Now().UnixNano() % int64(len(candidates)))
	default:
		for i := range candidates {
			if candidates[i].capacity > 0 {
				selected = i
				break
			}
		}
	}
	if candidates[selected].capacity < 0 {
		candidates[selected].capacity = 0
	}
	return candidates[selected].id, candidates[selected].path, candidates[selected].capacity, nil
}

func (s *Service) UploadLimit(ctx context.Context, owner string) (int64, error, error) {
	quotaRemaining, err := s.quotaRemaining(ctx, s.db.DB, owner)
	if err != nil {
		return 0, nil, err
	}
	if quotaRemaining == 0 {
		return 0, nil, errQuota
	}
	nodeID, _, safeFree, err := s.localPlacement(ctx)
	if err != nil {
		return 0, nil, err
	}
	if nodeID == "" {
		return 0, nil, errStorageFull
	}
	if safeFree <= 0 {
		return 0, nil, errStorageFull
	}
	if quotaRemaining <= safeFree {
		return quotaRemaining, errQuota, nil
	}
	return safeFree, errStorageFull, nil
}

func (s *Service) Upload(ctx context.Context, owner, name string, parent *string, r io.Reader, uniqueNameRequested bool, maxBytes int64, limitErr error) (Item, error) {
	parent = normalizeParent(parent)
	if !validName(name) {
		return Item{}, errInvalidName
	}

	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()

	refreshedLimit, refreshedLimitErr, err := s.UploadLimit(ctx, owner)
	if err != nil {
		return Item{}, err
	}
	if refreshedLimit <= maxBytes {
		maxBytes = refreshedLimit
		limitErr = refreshedLimitErr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, err
	}
	if err = s.checkParent(ctx, tx, owner, parent); err != nil {
		_ = tx.Rollback()
		return Item{}, err
	}
	if err = tx.Rollback(); err != nil {
		return Item{}, err
	}

	id := ulid.Make().String()
	nodeID, nodePath, available, err := s.localPlacement(ctx)
	if err != nil {
		return Item{}, err
	}
	var storedNodeID any
	if nodeID != "" {
		storedNodeID = nodeID
		if maxBytes > available {
			maxBytes = available
			limitErr = errStorageFull
		}
	}
	readLimit := maxBytes
	if readLimit < int64(^uint64(0)>>1) {
		readLimit++
	}
	counter := &countingReader{Reader: io.LimitReader(r, readLimit)}
	if err = s.storage.WriteLocal(ctx, nodePath, id, counter); err != nil {
		_ = s.storage.DeleteLocal(ctx, nodePath, id)
		return Item{}, err
	}
	cleanup := func() {
		_ = s.storage.DeleteLocal(ctx, nodePath, id)
	}
	size := counter.n
	if size > maxBytes {
		cleanup()
		return Item{}, limitErr
	}
	if nodeID != "" {
		physicalCapacity, physicalFree, statErr := s.storage.StatLocal(ctx, nodePath)
		if statErr != nil {
			cleanup()
			return Item{}, statErr
		}
		if physicalCapacity > 0 {
			reserve := physicalCapacity / 20
			if physicalCapacity%20 != 0 {
				reserve++
			}
			if physicalFree < reserve {
				cleanup()
				return Item{}, errStorageFull
			}
		}
	}

	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		cleanup()
		return Item{}, err
	}
	defer tx.Rollback()
	if err = s.checkParent(ctx, tx, owner, parent); err != nil {
		cleanup()
		return Item{}, err
	}

	var existingID string
	lookupErr := tx.QueryRowContext(
		ctx,
		`SELECT id FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`,
		owner,
		name,
		parent,
	).Scan(&existingID)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		cleanup()
		return Item{}, lookupErr
	}
	candidate := name
	if uniqueNameRequested || lookupErr == nil {
		candidate, err = uniqueName(ctx, tx, owner, parent, name, "")
		if err != nil {
			cleanup()
			return Item{}, err
		}
	}
	if err = s.checkQuota(ctx, tx, owner, size); err != nil {
		cleanup()
		return Item{}, err
	}
	if nodeID != "" {
		var capacity, used int64
		if err = tx.QueryRowContext(ctx, `SELECT capacity_bytes FROM storage_nodes WHERE id=? AND type='LOCAL' AND enabled=1`, nodeID).Scan(&capacity); err != nil {
			cleanup()
			return Item{}, errStorageFull
		}
		legacyID, legacyErr := legacyLocalStorageNodeID(ctx, tx)
		if legacyErr != nil {
			cleanup()
			return Item{}, legacyErr
		}
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT disk_key,MAX(size_bytes) size_bytes FROM my_storage_items WHERE kind='file' AND disk_key IS NOT NULL AND (storage_node_id=? OR (storage_node_id IS NULL AND ?=?)) GROUP BY disk_key)`, nodeID, nodeID, legacyID).Scan(&used); err != nil {
			cleanup()
			return Item{}, err
		}
		reserve := capacity / 20
		if capacity%20 != 0 {
			reserve++
		}
		if capacity-used-reserve < size {
			cleanup()
			return Item{}, errStorageFull
		}
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,parent_id,disk_key,storage_node_id,modified_at) VALUES(?,?,?,'file',?,?,?,?,?)`,
		id,
		owner,
		candidate,
		size,
		parent,
		id,
		storedNodeID,
		now,
	)
	if err != nil {
		cleanup()
		return Item{}, err
	}
	if err = tx.Commit(); err != nil {
		cleanup()
		return Item{}, err
	}
	return Item{ID: id, Name: candidate, Kind: "file", SizeBytes: size, ParentID: parent, ModifiedAt: now}, nil
}

type quotaQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func legacyLocalStorageNodeID(ctx context.Context, q quotaQuery) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, `SELECT id FROM storage_nodes WHERE type='LOCAL' ORDER BY CASE WHEN path LIKE '%/storage/default' THEN 0 ELSE 1 END, created_at ASC, id ASC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s *Service) quotaRemaining(ctx context.Context, q quotaQuery, owner string) (int64, error) {
	var quota int64
	var role string
	if err := q.QueryRowContext(ctx, `SELECT quota_bytes,role FROM users WHERE id=?`, owner).Scan(&quota, &role); err != nil {
		return 0, err
	}
	if role == "SUPER_ADMIN" || quota < 0 {
		return int64(^uint64(0) >> 1), nil
	}
	if quota == 0 {
		var gb string
		err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='default_user_quota_gb'`).Scan(&gb)
		if err == nil {
			var n int64
			_, _ = fmt.Sscan(gb, &n)
			if n > 0 && n <= int64(^uint64(0)>>1)/(1024*1024*1024) {
				quota = n * 1024 * 1024 * 1024
			} else {
				quota = -1
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			quota = -1
		} else {
			return 0, err
		}
	}
	if quota < 0 {
		return int64(^uint64(0) >> 1), nil
	}
	var used int64
	if err := q.QueryRowContext(
		ctx,
		`SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT MAX(size_bytes) AS size_bytes FROM my_storage_items WHERE owner_id=? AND kind='file' GROUP BY disk_key)`,
		owner,
	).Scan(&used); err != nil {
		return 0, err
	}
	if used >= quota {
		return 0, nil
	}
	return quota - used, nil
}

func (s *Service) checkQuota(ctx context.Context, tx quotaQuery, owner string, extra int64) error {
	remaining, err := s.quotaRemaining(ctx, tx, owner)
	if err != nil {
		return err
	}
	if extra > remaining {
		return errQuota
	}
	return nil
}

func (s *Service) Update(ctx context.Context, owner, id string, name *string, parent *string, changeParent bool) (Item, string, error) {
	if changeParent {
		parent = normalizeParent(parent)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, "", err
	}
	defer tx.Rollback()
	i, err := s.getTx(ctx, tx, owner, id)
	if err != nil {
		return i, "", err
	}
	newName := i.Name
	if name != nil {
		if !validName(*name) {
			return Item{}, "", errInvalidName
		}
		newName = *name
	}
	newParent := i.ParentID
	if changeParent {
		newParent = parent
	}
	if err = s.checkParent(ctx, tx, owner, newParent); err != nil {
		return Item{}, "", err
	}
	if i.Kind == "folder" && newParent != nil {
		var n int
		err = tx.QueryRowContext(
			ctx,
			`WITH RECURSIVE descendants(id) AS (SELECT id FROM my_storage_items WHERE parent_id=? AND owner_id=? UNION ALL SELECT m.id FROM my_storage_items m JOIN descendants d ON m.parent_id=d.id WHERE m.owner_id=?) SELECT COUNT(*) FROM descendants WHERE id=?`,
			id,
			owner,
			owner,
			*newParent,
		).Scan(&n)
		if err != nil {
			return Item{}, "", err
		}
		if n > 0 || *newParent == id {
			return Item{}, "", errMoveCycle
		}
	}
	var count int
	err = tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') AND id<>?`,
		owner,
		newName,
		newParent,
		id,
	).Scan(&count)
	if err != nil {
		return Item{}, "", err
	}
	if count > 0 {
		suggestion, e := uniqueName(ctx, tx, owner, newParent, newName, id)
		if e != nil {
			return Item{}, "", e
		}
		return Item{}, suggestion, errConflict
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`UPDATE my_storage_items SET name=?,parent_id=?,modified_at=? WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
		newName,
		newParent,
		now,
		id,
		owner,
	)
	if err != nil {
		return Item{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return Item{}, "", err
	}
	i.Name = newName
	i.ParentID = newParent
	i.ModifiedAt = now
	return i, "", nil
}

func (s *Service) getTx(ctx context.Context, tx quotaQuery, owner, id string) (Item, error) {
	var i Item
	err := tx.QueryRowContext(
		ctx,
		`SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
			CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END AS is_starred
		FROM my_storage_items i
		LEFT JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.id=? AND i.owner_id=? AND i.deleted_at IS NULL`,
		owner,
		id,
		owner,
	).Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.IsStarred)
	if errors.Is(err, sql.ErrNoRows) {
		return i, errNotFound
	}
	return i, err
}
func (s *Service) localPathForKey(ctx context.Context, key string) (string, error) {
	var nodePath *string
	err := s.db.QueryRowContext(ctx, `SELECT sn.path FROM my_storage_items i LEFT JOIN storage_nodes sn ON sn.id=i.storage_node_id WHERE i.disk_key=? ORDER BY i.storage_node_id IS NULL LIMIT 1`, key).Scan(&nodePath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if nodePath == nil {
		return "", nil
	}
	return *nodePath, nil
}

func (s *Service) Delete(ctx context.Context, owner, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.getTx(ctx, tx, owner, id); err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`WITH RECURSIVE descendants(id) AS (SELECT id FROM my_storage_items WHERE id=? AND owner_id=? UNION ALL SELECT m.id FROM my_storage_items m JOIN descendants d ON m.parent_id=d.id WHERE m.owner_id=?) UPDATE my_storage_items SET deleted_at=? WHERE id IN (SELECT id FROM descendants)`,
		id,
		owner,
		owner,
		now,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Duplicate(ctx context.Context, owner, id string) (Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback()
	root, err := s.getTx(ctx, tx, owner, id)
	if err != nil {
		return Item{}, err
	}
	name, err := uniqueName(ctx, tx, owner, root.ParentID, root.Name, "")
	if err != nil {
		return Item{}, err
	}
	root.Name = name
	var rows []Item
	err = loadTree(ctx, tx, owner, id, &rows)
	if err != nil {
		return Item{}, err
	}
	ids := map[string]string{id: ulid.Make().String()}
	for _, item := range rows {
		if item.ID != id {
			ids[item.ID] = ulid.Make().String()
		}
	}
	for _, item := range rows {
		parent := item.ParentID
		if item.ID == id {
			parent = root.ParentID
		} else if parent != nil {
			mapped := ids[*parent]
			parent = &mapped
		}
		newName := item.Name
		if item.ID == id {
			newName = name
		}
		var disk any
		if item.DiskKey != nil {
			disk = *item.DiskKey
		}
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,parent_id,disk_key,storage_node_id,modified_at) VALUES(?,?,?,?,?,?,?,(SELECT storage_node_id FROM my_storage_items WHERE id=?),?)`,
			ids[item.ID],
			owner,
			newName,
			item.Kind,
			item.SizeBytes,
			parent,
			disk,
			item.ID,
			time.Now().UTC(),
		)
		if err != nil {
			return Item{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Item{}, err
	}
	root.ID = ids[id]
	return root, nil
}

func loadTree(
	ctx context.Context,
	tx interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	owner, id string,
	out *[]Item,
) error {
	rows, err := tx.QueryContext(
		ctx,
		`WITH RECURSIVE tree(id,name,kind,size_bytes,parent_id,modified_at,disk_key) AS (SELECT id,name,kind,size_bytes,parent_id,modified_at,disk_key FROM my_storage_items WHERE id=? AND owner_id=? AND deleted_at IS NULL UNION ALL SELECT m.id,m.name,m.kind,m.size_bytes,m.parent_id,m.modified_at,m.disk_key FROM my_storage_items m JOIN tree t ON m.parent_id=t.id WHERE m.owner_id=? AND m.deleted_at IS NULL) SELECT id,name,kind,size_bytes,parent_id,modified_at,disk_key FROM tree`,
		id,
		owner,
		owner,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i Item
		if err = rows.Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey); err != nil {
			return err
		}
		*out = append(*out, i)
	}
	return rows.Err()
}

func (s *Service) Download(ctx context.Context, owner, id string) (Item, io.ReadCloser, error) {
	item, err := s.get(ctx, owner, id)
	if err != nil {
		return item, nil, err
	}
	if item.Kind == "file" {
		nodePath, err := s.localPathForKey(ctx, *item.DiskKey)
		if err != nil {
			return item, nil, err
		}
		r, err := s.storage.ReadLocal(ctx, nodePath, *item.DiskKey)
		return item, r, err
	}
	r, err := s.zipFolder(ctx, owner, item)
	return item, r, err
}

func (s *Service) zipFolder(ctx context.Context, owner string, root Item) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		var files []Item
		err := loadTree(ctx, s.db.DB, owner, root.ID, &files)
		if err == nil {
			for _, f := range files {
				if f.Kind != "file" {
					continue
				}
				nodePath, e := s.localPathForKey(ctx, *f.DiskKey)
				if e != nil {
					err = e
					break
				}
				r, e := s.storage.ReadLocal(ctx, nodePath, *f.DiskKey)
				if e != nil {
					err = e
					break
				}
				rel := f.Name
				parent := f.ParentID
				for parent != nil && *parent != root.ID {
					p, e := s.get(ctx, owner, *parent)
					if e != nil {
						err = e
						break
					}
					rel = path.Join(p.Name, rel)
					parent = p.ParentID
				}
				if err != nil {
					r.Close()
					break
				}
				w, e := zw.Create(rel)
				if e == nil {
					_, e = io.Copy(w, r)
				}
				r.Close()
				if e != nil {
					err = e
					break
				}
			}
		}
		closeErr := zw.Close()
		if err == nil {
			err = closeErr
		}
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

func (s *Service) MoveItem(ctx context.Context, userID string, itemID string, targetParentID *string) (*VFSNode, string, error) {
	targetParentID = normalizeParent(targetParentID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	var i VFSNode
	err = tx.QueryRowContext(
		ctx,
		`SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
			CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END AS is_starred
		FROM my_storage_items i
		LEFT JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.id=? AND i.owner_id=? AND i.deleted_at IS NULL`,
		userID,
		itemID,
		userID,
	).Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.IsStarred)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}

	if targetParentID != nil {
		if *targetParentID == itemID {
			return nil, "", ErrCircularMove
		}

		var targetKind string
		err = tx.QueryRowContext(
			ctx,
			`SELECT kind FROM my_storage_items WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
			*targetParentID,
			userID,
		).Scan(&targetKind)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrBadParent
		}
		if err != nil {
			return nil, "", err
		}
		if targetKind != "folder" {
			return nil, "", ErrBadParent
		}

		if i.Kind == "folder" {
			var n int
			err = tx.QueryRowContext(
				ctx,
				`WITH RECURSIVE descendants(id) AS (
					SELECT id FROM my_storage_items WHERE parent_id=? AND owner_id=? AND deleted_at IS NULL
					UNION ALL
					SELECT m.id FROM my_storage_items m JOIN descendants d ON m.parent_id=d.id WHERE m.owner_id=? AND m.deleted_at IS NULL
				) SELECT COUNT(*) FROM descendants WHERE id=?`,
				itemID,
				userID,
				userID,
				*targetParentID,
			).Scan(&n)
			if err != nil {
				return nil, "", err
			}
			if n > 0 {
				return nil, "", ErrCircularMove
			}
		}
	}

	var count int
	err = tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') AND id<>?`,
		userID,
		i.Name,
		targetParentID,
		itemID,
	).Scan(&count)
	if err != nil {
		return nil, "", err
	}
	if count > 0 {
		suggestion, e := uniqueName(ctx, tx, userID, targetParentID, i.Name, itemID)
		if e != nil {
			return nil, "", e
		}
		return nil, suggestion, ErrNameConflict
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`UPDATE my_storage_items SET parent_id=?, modified_at=? WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
		targetParentID,
		now,
		itemID,
		userID,
	)
	if err != nil {
		return nil, "", err
	}

	if err = tx.Commit(); err != nil {
		return nil, "", err
	}

	i.ParentID = targetParentID
	i.ModifiedAt = now
	return &i, "", nil
}

func (s *Service) ToggleStar(ctx context.Context, userID string, itemID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var existsID string
	err = tx.QueryRowContext(
		ctx,
		`SELECT id FROM my_storage_items WHERE id=? AND owner_id=? AND deleted_at IS NULL`,
		itemID,
		userID,
	).Scan(&existsID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}

	var starredID string
	err = tx.QueryRowContext(
		ctx,
		`SELECT id FROM vfs_starred WHERE user_id=? AND node_id=?`,
		userID,
		itemID,
	).Scan(&starredID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	if err == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM vfs_starred WHERE user_id=? AND node_id=?`, userID, itemID)
		if err != nil {
			return false, err
		}
		if err = tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	starID := ulid.Make().String()
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO vfs_starred(id, user_id, node_id, starred_at) VALUES(?, ?, ?, ?)`,
		starID,
		userID,
		itemID,
		now,
	)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) ListStarred(ctx context.Context, userID string) ([]VFSNode, error) {
	query := `
		SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key, 1 AS is_starred
		FROM my_storage_items i
		JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.owner_id = ? AND i.deleted_at IS NULL
		ORDER BY s.starred_at DESC
	`
	rows, err := s.db.DB.QueryContext(ctx, query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]VFSNode, 0)
	for rows.Next() {
		var i VFSNode
		if err = rows.Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.IsStarred); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (s *Service) ListFiles(ctx context.Context, owner string, parent *string) ([]VFSNode, error) {
	items, _, err := s.List(ctx, owner, parent)
	return items, err
}
