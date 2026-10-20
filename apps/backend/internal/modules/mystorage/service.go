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

	"github.com/cubycloud/backend/internal/database"
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
	errInvalidSize = errors.New("invalid upload size")
	errBadParent   = errors.New("parent is not a folder")
	errMoveCycle   = errors.New("cannot move folder into itself")

	ErrNotFound     = errNotFound
	ErrNameConflict = errors.New("name already in use")
	ErrCircularMove = errors.New("cannot move folder into itself or its descendant")
	ErrBadParent    = errBadParent
	ErrStorageFull  = errStorageFull
	ErrQuota        = errQuota
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
type LargestFileItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	SizeBytes  int64     `json:"size_bytes"`
	ParentID   *string   `json:"parent_id"`
	Location   string    `json:"location"`
	ModifiedAt time.Time `json:"modified_at"`
}

type LargestFilesResult struct {
	Items      []LargestFileItem `json:"items"`
	Total      int64             `json:"total"`
	HasMore    bool              `json:"has_more"`
	NextOffset int               `json:"next_offset"`
}

type StorageCategoryUsage struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Bytes  int64   `json:"bytes"`
	UsedGB float64 `json:"used_gb"`
}

type CategoryUsageResult struct {
	TotalBytes int64                  `json:"total_bytes"`
	TotalGB    float64                `json:"total_gb"`
	Categories []StorageCategoryUsage `json:"categories"`
}

type UploadIntentResult struct {
	Placement string `json:"placement"` // "local" | "s3"
	UploadID  string `json:"upload_id,omitempty"`
	URL       string `json:"url,omitempty"`
	Method    string `json:"method,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type DownloadURLResult struct {
	URL       *string `json:"url"`
	Filename  string  `json:"filename"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

type Service struct {
	db       *bun.DB
	storage  storage.StorageDriver
	uploadMu sync.Mutex
}

var cloudUploadMu sync.Mutex

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

func (s *Service) Get(ctx context.Context, owner, id string) (Item, error) {
	return s.get(ctx, owner, id)
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
	_ = database.RecordActivity(ctx, tx, owner, "upload", i.ID, name, "folder", "my-storage")
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
		used, err := storage.NodeUsedBytes(ctx, s.db.DB, candidates[i].id)
		if err != nil {
			return "", "", 0, err
		}
		reserve := candidates[i].capacity / 20
		if candidates[i].capacity%20 != 0 {
			reserve++
		}
		reserved := storage.GetNodeReserved(candidates[i].id)
		candidates[i].capacity -= used + reserve + reserved
		if candidates[i].capacity < 0 {
			candidates[i].capacity = 0
		}
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
	placement, err := storage.SelectPlacement(ctx, s.db, s.storage, 1)
	if err != nil {
		return Item{}, err
	}
	var storedNodeID any
	if placement.ID != "" {
		storedNodeID = placement.ID
		if maxBytes > placement.Available {
			maxBytes = placement.Available
			limitErr = errStorageFull
		}
	}
	var initialReserve int64
	if sz, ok := r.(interface{ Size() int64 }); ok {
		initialReserve = sz.Size()
	} else if lr, ok := r.(interface{ Len() int }); ok {
		initialReserve = int64(lr.Len())
	}
	res := storage.ReserveNode(placement.ID, initialReserve)
	defer res.Release()
	readLimit := maxBytes
	if readLimit < int64(^uint64(0)>>1) {
		readLimit++
	}
	counter := &countingReader{Reader: io.LimitReader(r, readLimit)}

	var cleanup func()
	if placement.Type == "S3" {
		cloud, ok := s.storage.(storage.CloudStorageDriver)
		if !ok {
			return Item{}, errors.New("cloud driver unavailable")
		}
		if err = cloud.WriteObject(ctx, placement.ID, id, counter, -1); err != nil {
			_ = cloud.DeleteObject(ctx, placement.ID, id)
			return Item{}, err
		}
		cleanup = func() {
			_ = cloud.DeleteObject(ctx, placement.ID, id)
		}
	} else {
		if err = s.storage.WriteLocal(ctx, placement.Path, id, counter); err != nil {
			_ = s.storage.DeleteLocal(ctx, placement.Path, id)
			return Item{}, err
		}
		cleanup = func() {
			_ = s.storage.DeleteLocal(ctx, placement.Path, id)
		}
	}
	size := counter.n
	if size > maxBytes {
		cleanup()
		return Item{}, limitErr
	}
	res.SetBytes(size)

	unlockNode := storage.LockNode(placement.ID)
	defer unlockNode()
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
	if placement.ID != "" && placement.Type == "LOCAL" {
		if err = storage.CheckNodeCapacityTx(ctx, tx, s.storage, placement.ID, size); err != nil {
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
	_ = database.RecordActivity(ctx, tx, owner, "upload", id, candidate, "file", "my-storage")
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
	if newName != i.Name || changeParent {
		_ = database.RecordActivity(ctx, tx, owner, "edit", id, newName, i.Kind, "my-storage")
	}
	if err = tx.Commit(); err != nil {
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
		r, err := storage.ReadBlob(ctx, s.db, s.storage, "my_storage_items", *item.DiskKey)
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
				r, e := storage.ReadBlob(ctx, s.db, s.storage, "my_storage_items", *f.DiskKey)
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

func (s *Service) BatchDownloadZip(ctx context.Context, owner string, itemIDs []string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		var err error
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

		for _, id := range itemIDs {
			item, e := s.get(ctx, owner, id)
			if e != nil {
				continue
			}
			if item.Kind == "file" {
				if item.DiskKey == nil {
					continue
				}
				r, e := storage.ReadBlob(ctx, s.db, s.storage, "my_storage_items", *item.DiskKey)
				if e != nil {
					err = e
					break
				}
				entryName := sanitizePath(item.Name)
				w, e := zw.Create(entryName)
				if e == nil {
					_, e = io.Copy(w, r)
				}
				r.Close()
				if e != nil {
					err = e
					break
				}
			} else if item.Kind == "folder" {
				var files []Item
				if e := loadTree(ctx, s.db.DB, owner, item.ID, &files); e != nil {
					err = e
					break
				}
				for _, f := range files {
					if f.Kind != "file" || f.DiskKey == nil {
						continue
					}
					r, e := storage.ReadBlob(ctx, s.db, s.storage, "my_storage_items", *f.DiskKey)
					if e != nil {
						err = e
						break
					}
					rel := f.Name
					parent := f.ParentID
					for parent != nil && *parent != item.ID {
						p, e := s.get(ctx, owner, *parent)
						if e != nil {
							err = e
							break
						}
						rel = path.Join(p.Name, rel)
						parent = p.ParentID
					}
					rel = path.Join(item.Name, rel)
					entryName := sanitizePath(rel)
					w, e := zw.Create(entryName)
					if e == nil {
						_, e = io.Copy(w, r)
					}
					r.Close()
					if e != nil {
						err = e
						break
					}
				}
				if err != nil {
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
	_ = database.RecordActivity(ctx, tx, userID, "edit", itemID, i.Name, i.Kind, "my-storage")

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

func (s *Service) ListLargestFiles(ctx context.Context, owner string, limit int, offset int) (LargestFilesResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	result := LargestFilesResult{
		Items:      make([]LargestFileItem, 0),
		Total:      0,
		HasMore:    false,
		NextOffset: offset,
	}

	var total int64
	err := s.db.DB.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM my_storage_items WHERE owner_id = ? AND kind = 'file' AND deleted_at IS NULL`,
		owner,
	).Scan(&total)
	if err != nil {
		return result, err
	}
	result.Total = total
	if total == 0 || int64(offset) >= total {
		return result, nil
	}

	folderRows, err := s.db.DB.QueryContext(
		ctx,
		`SELECT id, name, parent_id FROM my_storage_items WHERE owner_id = ? AND kind = 'folder' AND deleted_at IS NULL`,
		owner,
	)
	if err != nil {
		return result, err
	}
	defer folderRows.Close()

	folderMap := make(map[string]Folder)
	for folderRows.Next() {
		var f Folder
		if err := folderRows.Scan(&f.ID, &f.Name, &f.ParentID); err != nil {
			return result, err
		}
		folderMap[f.ID] = f
	}
	if err := folderRows.Err(); err != nil {
		return result, err
	}

	resolveLocation := func(parentID *string) string {
		if parentID == nil || *parentID == "" {
			return "My Storage"
		}
		var parts []string
		curr := parentID
		visited := make(map[string]bool)
		for curr != nil && *curr != "" {
			if visited[*curr] {
				break
			}
			visited[*curr] = true
			f, ok := folderMap[*curr]
			if !ok {
				break
			}
			parts = append(parts, f.Name)
			curr = f.ParentID
		}
		if len(parts) == 0 {
			return "My Storage"
		}
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		return "My Storage / " + strings.Join(parts, " / ")
	}

	fileRows, err := s.db.DB.QueryContext(
		ctx,
		`SELECT id, name, kind, size_bytes, parent_id, modified_at
		FROM my_storage_items
		WHERE owner_id = ? AND kind = 'file' AND deleted_at IS NULL
		ORDER BY size_bytes DESC, modified_at DESC, id ASC
		LIMIT ? OFFSET ?`,
		owner,
		limit,
		offset,
	)
	if err != nil {
		return result, err
	}
	defer fileRows.Close()

	items := make([]LargestFileItem, 0, limit)
	for fileRows.Next() {
		var item LargestFileItem
		if err := fileRows.Scan(
			&item.ID,
			&item.Name,
			&item.Kind,
			&item.SizeBytes,
			&item.ParentID,
			&item.ModifiedAt,
		); err != nil {
			return result, err
		}
		item.Location = resolveLocation(item.ParentID)
		items = append(items, item)
	}
	if err := fileRows.Err(); err != nil {
		return result, err
	}

	result.Items = items
	result.NextOffset = offset + len(items)
	result.HasMore = int64(result.NextOffset) < total

	return result, nil
}

func detectCategory(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	switch ext {
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "3gp", "ts", "mpg", "mpeg":
		return "video"
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "heic", "heif", "tiff", "tif", "ico", "avif":
		return "image"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv", "rtf", "odt", "ods", "odp", "md", "html", "htm", "xml", "json", "yaml", "yml":
		return "document"
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "tgz", "iso", "dmg":
		return "archive"
	default:
		return "other"
	}
}

func (s *Service) CategoryUsage(ctx context.Context, owner string) (CategoryUsageResult, error) {
	query := `
SELECT
	COALESCE(disk_key, id) AS k,
	MAX(size_bytes) AS size_bytes,
	MAX(CASE WHEN deleted_at IS NULL THEN name ELSE '' END) AS active_name,
	MAX(name) AS any_name,
	COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) AS active_count
FROM my_storage_items
WHERE owner_id = ? AND kind = 'file'
GROUP BY k`

	rows, err := s.db.DB.QueryContext(ctx, query, owner)
	if err != nil {
		return CategoryUsageResult{}, err
	}
	defer rows.Close()

	var (
		videoBytes    int64
		imageBytes    int64
		documentBytes int64
		archiveBytes  int64
		otherBytes    int64
		trashBytes    int64
		totalBytes    int64
	)

	for rows.Next() {
		var (
			k           string
			sizeBytes   int64
			activeName  string
			anyName     string
			activeCount int
		)
		if err := rows.Scan(&k, &sizeBytes, &activeName, &anyName, &activeCount); err != nil {
			return CategoryUsageResult{}, err
		}
		totalBytes += sizeBytes

		if activeCount == 0 {
			trashBytes += sizeBytes
			continue
		}

		name := activeName
		if name == "" {
			name = anyName
		}
		cat := detectCategory(name)
		switch cat {
		case "video":
			videoBytes += sizeBytes
		case "image":
			imageBytes += sizeBytes
		case "document":
			documentBytes += sizeBytes
		case "archive":
			archiveBytes += sizeBytes
		default:
			otherBytes += sizeBytes
		}
	}
	if err := rows.Err(); err != nil {
		return CategoryUsageResult{}, err
	}

	const gib = float64(1024 * 1024 * 1024)
	toGB := func(b int64) float64 {
		return float64(b) / gib
	}

	categories := []StorageCategoryUsage{
		{ID: "video", Label: "Video", Bytes: videoBytes, UsedGB: toGB(videoBytes)},
		{ID: "image", Label: "Gambar", Bytes: imageBytes, UsedGB: toGB(imageBytes)},
		{ID: "document", Label: "Dokumen", Bytes: documentBytes, UsedGB: toGB(documentBytes)},
		{ID: "archive", Label: "Arsip", Bytes: archiveBytes, UsedGB: toGB(archiveBytes)},
	}
	if otherBytes > 0 {
		categories = append(categories, StorageCategoryUsage{
			ID:     "other",
			Label:  "Lainnya",
			Bytes:  otherBytes,
			UsedGB: toGB(otherBytes),
		})
	}
	categories = append(categories, StorageCategoryUsage{
		ID:     "trash",
		Label:  "Trash",
		Bytes:  trashBytes,
		UsedGB: toGB(trashBytes),
	})

	return CategoryUsageResult{
		TotalBytes: totalBytes,
		TotalGB:    toGB(totalBytes),
		Categories: categories,
	}, nil
}

func (s *Service) UploadIntent(ctx context.Context, owner, filename string, size int64, parent *string, uniqueName bool) (*UploadIntentResult, error) {
	parent = normalizeParent(parent)
	if !validName(filename) {
		return nil, errInvalidName
	}
	if size < 0 {
		return nil, errInvalidSize
	}

	quotaRemaining, err := s.quotaRemaining(ctx, s.db.DB, owner)
	if err != nil {
		return nil, err
	}
	if quotaRemaining == 0 || (quotaRemaining > 0 && quotaRemaining < size) {
		return nil, errQuota
	}

	placement, err := storage.SelectPlacement(ctx, s.db, s.storage, size)
	if err != nil {
		return nil, errStorageFull
	}

	if placement.Type != "S3" {
		return &UploadIntentResult{
			Placement: "local",
		}, nil
	}

	cloud, ok := s.storage.(storage.CloudStorageDriver)
	if !ok {
		return &UploadIntentResult{Placement: "local"}, nil
	}

	uploadID := ulid.Make().String()
	urlExpiry := 15 * time.Minute
	putURL, err := cloud.PresignUpload(ctx, placement.ID, uploadID, urlExpiry)
	if err != nil {
		return nil, fmt.Errorf("presign upload: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)
	var parentVal any
	if parent != nil && *parent != "" {
		parentVal = *parent
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO pending_cloud_uploads (id, owner_id, storage_node_id, filename, size_bytes, parent_id, unique_name, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, uploadID, owner, placement.ID, filename, size, parentVal, uniqueName, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("record pending upload: %w", err)
	}

	return &UploadIntentResult{
		Placement: "s3",
		UploadID:  uploadID,
		URL:       putURL,
		Method:    "PUT",
		ExpiresAt: now.Add(urlExpiry).Format(time.RFC3339),
	}, nil
}

func (s *Service) SyncCloud(ctx context.Context, owner, uploadID string) (Item, error) {
	var storageNodeID, filename string
	var sizeBytes int64
	var parentID sql.NullString
	var uniqueNameRequested bool
	var expiresAt time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT storage_node_id, filename, size_bytes, parent_id, unique_name, expires_at
		FROM pending_cloud_uploads
		WHERE id = ? AND owner_id = ?
	`, uploadID, owner).Scan(&storageNodeID, &filename, &sizeBytes, &parentID, &uniqueNameRequested, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, errNotFound
	}
	if err != nil {
		return Item{}, err
	}

	var parent *string
	if parentID.Valid && parentID.String != "" {
		parent = &parentID.String
	}
	parent = normalizeParent(parent)

	cloud, ok := s.storage.(storage.CloudStorageDriver)
	if !ok {
		return Item{}, errors.New("cloud driver unavailable")
	}

	cloudUploadMu.Lock()
	defer cloudUploadMu.Unlock()
	var pending int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pending_cloud_uploads WHERE id=? AND owner_id=?`, uploadID, owner).Scan(&pending); err != nil {
		return Item{}, err
	}
	if pending == 0 {
		return Item{}, errNotFound
	}
	if !expiresAt.After(time.Now().UTC()) {
		return Item{}, errNotFound
	}
	actualSize, err := cloud.HeadObject(ctx, storageNodeID, uploadID)
	if err != nil {
		return Item{}, fmt.Errorf("cloud object verification failed: %w", err)
	}
	if actualSize < 0 || actualSize != sizeBytes {
		return Item{}, errInvalidSize
	}
	// AvailableOnNode clamps an overcommitted node to zero; reject even when
	// adding this upload's own reservation would otherwise make it look free.
	var capacity int64
	if err = s.db.QueryRowContext(ctx, `SELECT capacity_bytes FROM storage_nodes WHERE id=?`, storageNodeID).Scan(&capacity); err != nil {
		return Item{}, errStorageFull
	}
	if capacity <= 0 {
		capacity = 1000 * 1024 * 1024 * 1024
	} else {
		reserve := capacity / 20
		if capacity%20 != 0 {
			reserve++
		}
		capacity -= reserve
	}
	used, err := storage.NodeUsedBytes(ctx, s.db.DB, storageNodeID)
	if err != nil {
		return Item{}, errStorageFull
	}
	if used > capacity-storage.GetNodeReserved(storageNodeID) {
		return Item{}, errStorageFull
	}

	available, err := storage.AvailableOnNode(ctx, s.db, s.storage, storageNodeID)
	if err != nil {
		return Item{}, errStorageFull
	}
	if sizeBytes > 0 {
		available += sizeBytes
	}
	if actualSize > available {
		return Item{}, errStorageFull
	}

	// Keep the SQLite connection after a failed COMMIT so deferred constraint
	// errors can be rolled back before database/sql returns it to the pool.
	tx, err := s.db.DB.Conn(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Close()
	if _, err = tx.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return Item{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = tx.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	if err = s.checkQuota(ctx, tx, owner, actualSize); err != nil {
		return Item{}, err
	}



	if err = s.checkParent(ctx, tx, owner, parent); err != nil {
		return Item{}, err
	}

	var existingID string
	lookupErr := tx.QueryRowContext(
		ctx,
		`SELECT id FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`,
		owner,
		filename,
		parent,
	).Scan(&existingID)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return Item{}, lookupErr
	}

	candidate := filename
	if uniqueNameRequested || lookupErr == nil {
		candidate, err = uniqueName(ctx, tx, owner, parent, filename, "")
		if err != nil {
			return Item{}, err
		}
	} else if lookupErr == nil {
		return Item{}, errConflict
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		 VALUES (?, ?, ?, 'file', ?, ?, ?, ?, ?)`,
		uploadID,
		owner,
		candidate,
		sizeBytes,
		parent,
		uploadID,
		storageNodeID,
		now,
	)
	if err != nil {
		return Item{}, err
	}

	_ = database.RecordActivity(ctx, tx, owner, "upload", uploadID, candidate, "file", "my-storage")
	if _, err = tx.ExecContext(ctx, `DELETE FROM pending_cloud_uploads WHERE id = ? AND owner_id = ?`, uploadID, owner); err != nil {
		return Item{}, err
	}

	if _, err = tx.ExecContext(ctx, `COMMIT`); err != nil {
		return Item{}, err
	}
	committed = true

	return Item{
		ID:         uploadID,
		Name:       candidate,
		Kind:       "file",
		SizeBytes:  sizeBytes,
		ParentID:   parent,
		ModifiedAt: now,
	}, nil
}

// Keep a blob that is still referenced by either drive; only orphan uploads
// belong to the pending intent's cancellation/expiry cleanup.
func (s *Service) deleteUnreferencedCloudUpload(ctx context.Context, cloud storage.CloudStorageDriver, nodeID, uploadID string) error {
	var referenced bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM my_storage_items WHERE disk_key=? AND storage_node_id=?
			UNION ALL
			SELECT 1 FROM organization_items WHERE disk_key=? AND storage_node_id=?
		)
	`, uploadID, nodeID, uploadID, nodeID).Scan(&referenced)
	if err != nil || referenced {
		return err
	}
	return cloud.DeleteObject(ctx, nodeID, uploadID)
}
func (s *Service) CancelUploadIntent(ctx context.Context, owner, uploadID string) (bool, error) {
	cloudUploadMu.Lock()
	defer cloudUploadMu.Unlock()

	var nodeID string
	err := s.db.QueryRowContext(ctx, `SELECT storage_node_id FROM pending_cloud_uploads WHERE id=? AND owner_id=?`, uploadID, owner).Scan(&nodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	cloud, ok := s.storage.(storage.CloudStorageDriver)
	if !ok {
		return false, errors.New("cloud driver unavailable")
	}
	if err = s.deleteUnreferencedCloudUpload(ctx, cloud, nodeID, uploadID); err != nil {
		return false, fmt.Errorf("delete cloud upload object: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM pending_cloud_uploads WHERE id=? AND owner_id=?`, uploadID, owner)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	return deleted > 0, err
}

func (s *Service) CleanupExpiredPendingCloudUploads(ctx context.Context) (int64, error) {
	cloudUploadMu.Lock()
	defer cloudUploadMu.Unlock()

	rows, err := s.db.QueryContext(ctx, `SELECT id, storage_node_id FROM pending_cloud_uploads WHERE expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, err
	}
	type expiredUpload struct{ id, nodeID string }
	var uploads []expiredUpload
	for rows.Next() {
		var upload expiredUpload
		if err = rows.Scan(&upload.id, &upload.nodeID); err != nil {
			break
		}
		uploads = append(uploads, upload)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return 0, err
	}

	cloud, ok := s.storage.(storage.CloudStorageDriver)
	if !ok && len(uploads) > 0 {
		return 0, errors.New("cloud driver unavailable")
	}
	var deleted int64
	var firstErr error
	for _, upload := range uploads {
		if err = s.deleteUnreferencedCloudUpload(ctx, cloud, upload.nodeID, upload.id); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("delete expired cloud upload %s: %w", upload.id, err)
			}
			continue
		}
		result, deleteErr := s.db.ExecContext(ctx, `DELETE FROM pending_cloud_uploads WHERE id=? AND expires_at <= CURRENT_TIMESTAMP`, upload.id)
		if deleteErr != nil {
			if firstErr == nil {
				firstErr = deleteErr
			}
			continue
		}
		n, countErr := result.RowsAffected()
		if countErr != nil {
			if firstErr == nil {
				firstErr = countErr
			}
			continue
		}
		deleted += n
	}
	return deleted, firstErr
}

func (s *Service) GetDownloadURL(ctx context.Context, owner, id string) (*DownloadURLResult, error) {
	item, err := s.get(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if item.Kind == "folder" {
		return &DownloadURLResult{
			URL:      nil,
			Filename: item.Name + ".zip",
		}, nil
	}

	var nodeType sql.NullString
	var nodeID sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT sn.id, sn.type
		FROM my_storage_items i
		JOIN storage_nodes sn ON sn.id = i.storage_node_id
		WHERE i.id = ? AND i.owner_id = ? AND i.deleted_at IS NULL
	`, id, owner).Scan(&nodeID, &nodeType)
	if errors.Is(err, sql.ErrNoRows) || !nodeType.Valid || nodeType.String != "S3" {
		return &DownloadURLResult{
			URL:      nil,
			Filename: item.Name,
		}, nil
	}

	cloud, ok := s.storage.(storage.CloudStorageDriver)
	if !ok {
		return &DownloadURLResult{
			URL:      nil,
			Filename: item.Name,
		}, nil
	}

	expiry := 15 * time.Minute
	presignedURL, err := cloud.PresignDownload(ctx, nodeID.String, *item.DiskKey, item.Name, expiry)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(expiry).Format(time.RFC3339)
	return &DownloadURLResult{
		URL:       &presignedURL,
		Filename:  item.Name,
		ExpiresAt: &expiresAt,
	}, nil
}
