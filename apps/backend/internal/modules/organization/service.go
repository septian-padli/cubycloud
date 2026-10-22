package organization

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
	errNotFound     = errors.New("item not found")
	errConflict     = errors.New("name conflict")
	errStorageFull  = errors.New("STORAGE_POOL_FULL")
	errInvalidName  = errors.New("invalid name")
	errBadParent    = errors.New("parent is not a folder")
	errMoveCycle    = errors.New("cannot move folder into itself")
	ErrNotFound     = errNotFound
	ErrNameConflict = errors.New("name already in use")
	ErrCircularMove = errors.New("cannot move folder into itself or its descendant")
	ErrBadParent    = errBadParent
)

type Item struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Kind             string    `json:"kind"`
	SizeBytes        int64     `json:"size_bytes"`
	ParentID         *string   `json:"parent_id"`
	ModifiedAt       time.Time `json:"modified_at"`
	IsStarred        bool      `json:"is_starred"`
	StorageAvailable bool      `json:"storage_available"`
	DiskKey          *string   `json:"-"`
	CreatedBy        string    `json:"created_by,omitempty"`
}

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

func (s *Service) List(ctx context.Context, userID string, parent *string) ([]Item, []Folder, error) {
	query := `SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
		CASE
			WHEN u.id IS NULL OR u.deleted_at IS NOT NULL THEN 'deleted user'
			ELSE u.username
		END AS created_by,
		CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END AS is_starred, COALESCE(i.storage_node_id,'')
		FROM organization_items i
		LEFT JOIN users u ON u.id = i.created_by
		LEFT JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.deleted_at IS NULL AND `
	args := []any{userID}
	if parent == nil || *parent == "" {
		query += `i.parent_id IS NULL`
	} else {
		parentItem, err := s.get(ctx, *parent)
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
	type listedItem struct {
		item   Item
		nodeID string
	}
	listed := make([]listedItem, 0)
	for rows.Next() {
		var record listedItem
		if err = rows.Scan(&record.item.ID, &record.item.Name, &record.item.Kind, &record.item.SizeBytes, &record.item.ParentID, &record.item.ModifiedAt, &record.item.DiskKey, &record.item.CreatedBy, &record.item.IsStarred, &record.nodeID); err != nil {
			return nil, nil, err
		}
		listed = append(listed, record)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, nil, err
	}
	items := make([]Item, 0, len(listed))
	availability := make(map[string]bool)
	for _, record := range listed {
		item := record.item
		if item.Kind != "folder" {
			available, ok := availability[record.nodeID]
			if !ok {
				available = storage.NodeAvailable(ctx, s.storage, record.nodeID)
				availability[record.nodeID] = available
			}
			item.StorageAvailable = available
		} else {
			item.StorageAvailable = true
		}
		items = append(items, item)
	}
	return items, nil, nil
}

func (s *Service) Folders(ctx context.Context) ([]Folder, error) {
	rows, err := s.db.DB.QueryContext(
		ctx,
		`SELECT id, name, parent_id FROM organization_items WHERE kind='folder' AND deleted_at IS NULL ORDER BY name COLLATE NOCASE`,
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

func (s *Service) Breadcrumbs(ctx context.Context, parent *string) ([]Folder, error) {
	if parent == nil || *parent == "" {
		return []Folder{}, nil
	}
	var out []Folder
	curr := parent
	for curr != nil && *curr != "" {
		var f Folder
		err := s.db.QueryRowContext(
			ctx,
			`SELECT id, name, parent_id FROM organization_items WHERE id=? AND deleted_at IS NULL`,
			*curr,
		).Scan(&f.ID, &f.Name, &f.ParentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return nil, err
		}
		out = append([]Folder{f}, out...)
		curr = f.ParentID
	}
	return out, nil
}

func (s *Service) CreateFolder(ctx context.Context, userID, name string, parent *string, uniqueNameRequested bool) (Item, string, error) {
	parent = normalizeParent(parent)
	if !validName(name) {
		return Item{}, "", errInvalidName
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, "", err
	}
	defer tx.Rollback()

	if err = s.checkParent(ctx, tx, parent); err != nil {
		return Item{}, "", err
	}

	var count int
	err = tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`,
		name,
		parent,
	).Scan(&count)
	if err != nil {
		return Item{}, "", err
	}
	if count > 0 {
		var existing Item
		_ = tx.QueryRowContext(
			ctx,
			`SELECT id, kind FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') LIMIT 1`,
			name,
			parent,
		).Scan(&existing.ID, &existing.Kind)
		suggestion, e := uniqueName(ctx, tx, parent, name, "")
		if e != nil {
			return Item{}, "", e
		}
		if !uniqueNameRequested {
			return existing, suggestion, errConflict
		}
		name = suggestion
	}

	id := ulid.Make().String()
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, modified_at) VALUES (?, ?, ?, 'folder', 0, ?, ?)`,
		id,
		userID,
		name,
		parent,
		now,
	)
	if err != nil {
		return Item{}, "", err
	}
	_ = database.RecordActivity(ctx, tx, userID, "upload", id, name, "folder", "organization")
	if err = tx.Commit(); err != nil {
		return Item{}, "", err
	}
	return Item{
		ID:         id,
		Name:       name,
		Kind:       "folder",
		SizeBytes:  0,
		ParentID:   parent,
		ModifiedAt: now,
		CreatedBy:  userID,
	}, "", nil
}

type localNode struct {
	id       string
	path     string
	capacity int64
	enabled  bool
}

func (s *Service) localPlacement(ctx context.Context) (string, string, int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(path,''), capacity_bytes, enabled FROM storage_nodes WHERE type='LOCAL' ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return "", "", 0, err
	}
	defer rows.Close()
	var all []localNode
	for rows.Next() {
		var n localNode
		if err = rows.Scan(&n.id, &n.path, &n.capacity, &n.enabled); err != nil {
			return "", "", 0, err
		}
		all = append(all, n)
	}
	if len(all) == 0 {
		return "", "", 0, nil
	}

	var legacyID string
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

func (s *Service) UploadLimit(ctx context.Context) (int64, error, error) {
	nodeID, _, safeFree, err := s.localPlacement(ctx)
	if err != nil {
		return 0, nil, err
	}
	if nodeID == "" || safeFree <= 0 {
		return 0, nil, errStorageFull
	}
	return safeFree, errStorageFull, nil
}

type countingReader struct {
	io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.n += int64(n)
	return n, err
}

func (s *Service) Upload(ctx context.Context, userID, name string, parent *string, r io.Reader, uniqueNameRequested bool, maxBytes int64, limitErr error) (Item, error) {
	parent = normalizeParent(parent)
	if !validName(name) {
		return Item{}, errInvalidName
	}

	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()

	refreshedLimit, refreshedLimitErr, err := s.UploadLimit(ctx)
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
	if err = s.checkParent(ctx, tx, parent); err != nil {
		_ = tx.Rollback()
		return Item{}, err
	}
	_ = tx.Rollback()

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

	var existingID string
	var existingKind string
	err = tx.QueryRowContext(
		ctx,
		`SELECT id, kind FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') LIMIT 1`,
		name,
		parent,
	).Scan(&existingID, &existingKind)
	hasConflict := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		cleanup()
		return Item{}, err
	}

	candidate := name
	if hasConflict {
		if !uniqueNameRequested {
			cleanup()
			suggestion, _ := uniqueName(ctx, tx, parent, name, "")
			return Item{ID: existingID, Kind: existingKind}, fmt.Errorf("%w: %s", errConflict, suggestion)
		}
		candidate, err = uniqueName(ctx, tx, parent, name, "")
		if err != nil {
			cleanup()
			return Item{}, err
		}
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
		`INSERT INTO organization_items(id, created_by, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at) VALUES(?, ?, ?, 'file', ?, ?, ?, ?, ?)`,
		id,
		userID,
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
	_ = database.RecordActivity(ctx, tx, userID, "upload", id, candidate, "file", "organization")
	if err = tx.Commit(); err != nil {
		cleanup()
		return Item{}, err
	}

	return Item{
		ID:         id,
		Name:       candidate,
		Kind:       "file",
		SizeBytes:  size,
		ParentID:   parent,
		ModifiedAt: now,
		CreatedBy:  userID,
	}, nil
}

func (s *Service) get(ctx context.Context, id string) (Item, error) {
	return s.getTx(ctx, s.db.DB, id)
}

func (s *Service) Get(ctx context.Context, userID, id string) (Item, error) {
	var i Item
	err := s.db.DB.QueryRowContext(
		ctx,
		`SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
			CASE
				WHEN u.id IS NULL OR u.deleted_at IS NOT NULL THEN 'deleted user'
				ELSE u.username
			END AS created_by,
			CASE WHEN s.id IS NOT NULL THEN 1 ELSE 0 END AS is_starred
		FROM organization_items i
		LEFT JOIN users u ON u.id = i.created_by
		LEFT JOIN vfs_starred s ON s.node_id = i.id AND s.user_id = ?
		WHERE i.id=? AND i.deleted_at IS NULL`,
		userID,
		id,
	).Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.CreatedBy, &i.IsStarred)
	if errors.Is(err, sql.ErrNoRows) {
		return i, errNotFound
	}
	return i, err
}

type queryRunner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Service) getTx(ctx context.Context, q queryRunner, id string) (Item, error) {
	var i Item
	err := q.QueryRowContext(
		ctx,
		`SELECT i.id, i.name, i.kind, i.size_bytes, i.parent_id, i.modified_at, i.disk_key,
			CASE
				WHEN u.id IS NULL OR u.deleted_at IS NOT NULL THEN 'deleted user'
				ELSE u.username
			END AS created_by
		FROM organization_items i
		LEFT JOIN users u ON u.id = i.created_by
		WHERE i.id=? AND i.deleted_at IS NULL`,
		id,
	).Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return i, errNotFound
	}
	return i, err
}

func (s *Service) localPathForKey(ctx context.Context, key string) (string, error) {
	var nodePath *string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT sn.path FROM organization_items i LEFT JOIN storage_nodes sn ON sn.id=i.storage_node_id WHERE i.disk_key=? ORDER BY i.storage_node_id IS NULL LIMIT 1`,
		key,
	).Scan(&nodePath)
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

func (s *Service) Download(ctx context.Context, id string) (Item, io.ReadCloser, error) {
	item, err := s.get(ctx, id)
	if err != nil {
		return item, nil, err
	}
	if item.Kind == "file" {
		r, err := storage.ReadBlob(ctx, s.db, s.storage, "organization_items", *item.DiskKey)
		return item, r, err
	}
	r, err := s.zipFolder(ctx, item)
	return item, r, err
}

func loadTree(ctx context.Context, q queryRunner, rootID string, out *[]Item) error {
	rows, err := q.QueryContext(
		ctx,
		`WITH RECURSIVE descendants(id, name, kind, size_bytes, parent_id, modified_at, disk_key, created_by) AS (
			SELECT id, name, kind, size_bytes, parent_id, modified_at, disk_key, created_by FROM organization_items WHERE id=? AND deleted_at IS NULL
			UNION ALL
			SELECT m.id, m.name, m.kind, m.size_bytes, m.parent_id, m.modified_at, m.disk_key, m.created_by
			FROM organization_items m JOIN descendants d ON m.parent_id=d.id WHERE m.deleted_at IS NULL
		) SELECT id, name, kind, size_bytes, parent_id, modified_at, disk_key, created_by FROM descendants`,
		rootID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i Item
		if err = rows.Scan(&i.ID, &i.Name, &i.Kind, &i.SizeBytes, &i.ParentID, &i.ModifiedAt, &i.DiskKey, &i.CreatedBy); err != nil {
			return err
		}
		*out = append(*out, i)
	}
	return rows.Err()
}

func (s *Service) zipFolder(ctx context.Context, root Item) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		var files []Item
		err := loadTree(ctx, s.db.DB, root.ID, &files)
		if err == nil {
			for _, f := range files {
				if f.Kind != "file" {
					continue
				}
				r, e := storage.ReadBlob(ctx, s.db, s.storage, "organization_items", *f.DiskKey)
				if e != nil {
					err = e
					break
				}
				rel := f.Name
				parent := f.ParentID
				for parent != nil && *parent != root.ID {
					p, e := s.get(ctx, *parent)
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

func (s *Service) BatchDownloadZip(ctx context.Context, itemIDs []string) (io.ReadCloser, error) {
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
			item, e := s.get(ctx, id)
			if e != nil {
				continue
			}
			if item.Kind == "file" {
				r, e := storage.ReadBlob(ctx, s.db, s.storage, "organization_items", *item.DiskKey)
				if e != nil {
					err = e
					break
				}
				entryPath := sanitizePath(item.Name)
				w, e := zw.Create(entryPath)
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
				if e := loadTree(ctx, s.db.DB, item.ID, &files); e != nil {
					err = e
					break
				}
				basePrefix := sanitizePath(item.Name)
				for _, f := range files {
					if f.Kind != "file" {
						continue
					}
					r, e := storage.ReadBlob(ctx, s.db, s.storage, "organization_items", *f.DiskKey)
					if e != nil {
						err = e
						break
					}
					rel := f.Name
					parent := f.ParentID
					for parent != nil && *parent != item.ID {
						p, e := s.get(ctx, *parent)
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
					entryPath := path.Join(basePrefix, rel)
					w, e := zw.Create(entryPath)
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

func (s *Service) Update(ctx context.Context, userID, id string, name *string, parent *string, changeParent bool) (Item, string, error) {
	if changeParent {
		parent = normalizeParent(parent)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, "", err
	}
	defer tx.Rollback()
	i, err := s.getTx(ctx, tx, id)
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
	if err = s.checkParent(ctx, tx, newParent); err != nil {
		return Item{}, "", err
	}
	if i.Kind == "folder" && newParent != nil {
		var n int
		err = tx.QueryRowContext(
			ctx,
			`WITH RECURSIVE descendants(id) AS (
				SELECT id FROM organization_items WHERE parent_id=?
				UNION ALL
				SELECT m.id FROM organization_items m JOIN descendants d ON m.parent_id=d.id
			) SELECT COUNT(*) FROM descendants WHERE id=?`,
			id,
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
		`SELECT COUNT(*) FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') AND id<>?`,
		newName,
		newParent,
		id,
	).Scan(&count)
	if err != nil {
		return Item{}, "", err
	}
	if count > 0 {
		suggestion, e := uniqueName(ctx, tx, newParent, newName, id)
		if e != nil {
			return Item{}, "", e
		}
		return Item{}, suggestion, errConflict
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`UPDATE organization_items SET name=?, parent_id=?, modified_at=? WHERE id=? AND deleted_at IS NULL`,
		newName,
		newParent,
		now,
		id,
	)
	if err != nil {
		return Item{}, "", err
	}
	if (name != nil && *name != i.Name) || changeParent {
		_ = database.RecordActivity(ctx, tx, userID, "edit", id, newName, i.Kind, "organization")
	}
	if err = tx.Commit(); err != nil {
		return Item{}, "", err
	}
	i.Name = newName
	i.ParentID = newParent
	i.ModifiedAt = now
	return i, "", nil
}

func (s *Service) MoveItem(ctx context.Context, userID, id string, targetParentID *string) (Item, string, error) {
	return s.Update(ctx, userID, id, nil, targetParentID, true)
}

func (s *Service) RecordAccess(ctx context.Context, userID, itemID string) error {
	return database.RecordRecentAccess(ctx, s.db.DB, userID, itemID, "organization")
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.getTx(ctx, tx, id); err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`WITH RECURSIVE descendants(id) AS (
			SELECT id FROM organization_items WHERE id=?
			UNION ALL
			SELECT m.id FROM organization_items m JOIN descendants d ON m.parent_id=d.id
		) UPDATE organization_items SET deleted_at=?, deleted_by=? WHERE id IN (SELECT id FROM descendants)`,
		id,
		now,
		userID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Duplicate(ctx context.Context, userID, id string) (Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback()
	root, err := s.getTx(ctx, tx, id)
	if err != nil {
		return Item{}, err
	}
	name, err := uniqueName(ctx, tx, root.ParentID, root.Name, "")
	if err != nil {
		return Item{}, err
	}
	newID := ulid.Make().String()
	now := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO organization_items(id, created_by, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, (SELECT storage_node_id FROM organization_items WHERE id=?), ?)`,
		newID,
		userID,
		name,
		root.Kind,
		root.SizeBytes,
		root.ParentID,
		root.DiskKey,
		root.ID,
		now,
	)
	if err != nil {
		return Item{}, err
	}
	if root.Kind == "folder" {
		var children []Item
		if err = loadTree(ctx, tx, root.ID, &children); err != nil {
			return Item{}, err
		}
		idMap := map[string]string{root.ID: newID}
		for _, c := range children {
			if c.ID == root.ID {
				continue
			}
			childID := ulid.Make().String()
			idMap[c.ID] = childID
			parentID := idMap[*c.ParentID]
			_, err = tx.ExecContext(
				ctx,
				`INSERT INTO organization_items(id, created_by, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
				VALUES(?, ?, ?, ?, ?, ?, ?, (SELECT storage_node_id FROM organization_items WHERE id=?), ?)`,
				childID,
				userID,
				c.Name,
				c.Kind,
				c.SizeBytes,
				parentID,
				c.DiskKey,
				c.ID,
				now,
			)
			if err != nil {
				return Item{}, err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return Item{}, err
	}
	root.ID = newID
	root.Name = name
	root.ModifiedAt = now
	root.CreatedBy = userID
	return root, nil
}

func (s *Service) ToggleStar(ctx context.Context, userID, itemID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := s.getTx(ctx, tx, itemID); err != nil {
		return false, err
	}

	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM vfs_starred WHERE user_id=? AND node_id=?`, userID, itemID).Scan(&existing)
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

func (s *Service) checkParent(ctx context.Context, q queryRunner, parent *string) error {
	if parent == nil || *parent == "" {
		return nil
	}
	var kind string
	err := q.QueryRowContext(
		ctx,
		`SELECT kind FROM organization_items WHERE id=? AND deleted_at IS NULL`,
		*parent,
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

func normalizeParent(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

func validName(name string) bool {
	clean := strings.TrimSpace(name)
	return clean != "" && !strings.ContainsAny(clean, "/\\")
}

func uniqueName(ctx context.Context, q queryRunner, parent *string, name, excludeID string) (string, error) {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	count := 1
	for {
		candidate := fmt.Sprintf("%s (%d)%s", base, count, ext)
		var c int
		err := q.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'') AND id<>?`,
			candidate,
			parent,
			excludeID,
		).Scan(&c)
		if err != nil {
			return "", err
		}
		if c == 0 {
			return candidate, nil
		}
		count++
	}
}
