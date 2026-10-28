package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// QueryRunner abstracts query execution for *bun.DB, *sql.DB, and *sql.Tx.
type QueryRunner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

var (
	resMu      sync.Mutex
	activeRes  = make(map[string]int64)
	resSeq     int64
	trackedRes = make(map[int64]*reservationEntry)

	nodeMuLocks sync.Map // map[string]*sync.Mutex
)

type reservationEntry struct {
	id     int64
	nodeID string
	bytes  int64
}

// NodeReservation tracks in-flight upload reservation lifecycle for a storage node.
type NodeReservation struct {
	id     int64
	nodeID string
	bytes  int64
	closed bool
	mu     sync.Mutex
}

// LockNode serializes node capacity checks and commits across modules.
func LockNode(nodeID string) func() {
	if nodeID == "" {
		return func() {}
	}
	val, _ := nodeMuLocks.LoadOrStore(nodeID, &sync.Mutex{})
	mu := val.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ReserveCapacity tracks bytes reserved on a node until the returned cleanup/done callback is executed.
func ReserveCapacity(nodeID string, size int64) func() {
	res := ReserveNode(nodeID, size)
	return res.Release
}

// ReserveNode creates a stateful in-flight reservation on nodeID that can be updated or released.
func ReserveNode(nodeID string, initialBytes int64) *NodeReservation {
	if nodeID == "" {
		return &NodeReservation{closed: true}
	}
	if initialBytes < 0 {
		initialBytes = 0
	}
	resMu.Lock()
	defer resMu.Unlock()

	resSeq++
	id := resSeq
	if initialBytes > 0 {
		activeRes[nodeID] += initialBytes
	}
	trackedRes[id] = &reservationEntry{
		id:     id,
		nodeID: nodeID,
		bytes:  initialBytes,
	}

	return &NodeReservation{
		id:     id,
		nodeID: nodeID,
		bytes:  initialBytes,
	}
}

// SetBytes updates the reserved bytes for this reservation.
func (r *NodeReservation) SetBytes(newBytes int64) {
	if r == nil {
		return
	}
	if newBytes < 0 {
		newBytes = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.nodeID == "" {
		return
	}

	resMu.Lock()
	defer resMu.Unlock()

	diff := newBytes - r.bytes
	r.bytes = newBytes
	activeRes[r.nodeID] += diff
	if activeRes[r.nodeID] <= 0 {
		delete(activeRes, r.nodeID)
	}
	if entry, ok := trackedRes[r.id]; ok {
		entry.bytes = newBytes
	}
}

// Release releases this reservation.
func (r *NodeReservation) Release() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	if r.nodeID == "" {
		return
	}

	resMu.Lock()
	defer resMu.Unlock()

	if r.bytes > 0 {
		activeRes[r.nodeID] -= r.bytes
		if activeRes[r.nodeID] <= 0 {
			delete(activeRes, r.nodeID)
		}
	}
	delete(trackedRes, r.id)
}

// GetNodeReserved returns active in-flight reserved bytes on a storage node.
func GetNodeReserved(nodeID string) int64 {
	if nodeID == "" {
		return 0
	}
	resMu.Lock()
	defer resMu.Unlock()
	return activeRes[nodeID]
}

// LegacyLocalNodeID returns the ID of the legacy/default LOCAL storage node (for records where storage_node_id IS NULL).
func LegacyLocalNodeID(ctx context.Context, q QueryRunner) (string, error) {
	var legacy string
	rows, err := q.QueryContext(ctx, `SELECT id, COALESCE(path,'') FROM storage_nodes WHERE type='LOCAL' ORDER BY created_at, id`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id, p string
		if err = rows.Scan(&id, &p); err != nil {
			return "", err
		}
		if legacy == "" || strings.HasSuffix(filepath.ToSlash(p), "/storage/default") {
			legacy = id
		}
	}
	return legacy, rows.Err()
}

// NodeUsedBytes returns the total bytes used on a storage node, combining
// my_storage_items and organization_items (grouped by disk_key for deduplication)
// plus unexpired pending_cloud_uploads.
func NodeUsedBytes(ctx context.Context, q QueryRunner, nodeID string) (int64, error) {
	if nodeID == "" {
		return 0, nil
	}
	legacy, err := LegacyLocalNodeID(ctx, q)
	if err != nil {
		return 0, err
	}
	var used int64
	err = q.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes),0) FROM (SELECT disk_key,MAX(size_bytes) size_bytes FROM (SELECT disk_key,size_bytes FROM my_storage_items WHERE kind='file' AND disk_key IS NOT NULL AND (storage_node_id=? OR (storage_node_id IS NULL AND ?=?)) UNION ALL SELECT disk_key,size_bytes FROM organization_items WHERE kind='file' AND disk_key IS NOT NULL AND (storage_node_id=? OR (storage_node_id IS NULL AND ?=?))) GROUP BY disk_key)`, nodeID, nodeID, legacy, nodeID, nodeID, legacy).Scan(&used)
	if err != nil {
		return 0, err
	}

	var pending int64
	err = q.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes),0) FROM pending_cloud_uploads WHERE storage_node_id=? AND expires_at>?`, nodeID, time.Now().UTC()).Scan(&pending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		if strings.Contains(err.Error(), "no such table") {
			return used, nil
		}
		return 0, err
	}

	return used + pending, nil
}

// CheckNodeCapacityTx verifies within a database transaction that a LOCAL storage node
// has enough remaining configured capacity for additionalBytes, taking into account
// the combined usage and active physical disk safety threshold.
func CheckNodeCapacityTx(ctx context.Context, tx QueryRunner, driver StorageDriver, nodeID string, additionalBytes int64) error {
	if nodeID == "" {
		return nil
	}
	var nodeType, nodePath string
	var capacity int64
	var enabled bool
	err := tx.QueryRowContext(ctx, `SELECT type, COALESCE(path,''), capacity_bytes, enabled FROM storage_nodes WHERE id=?`, nodeID).Scan(&nodeType, &nodePath, &capacity, &enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPoolFull
		}
		return err
	}
	if !enabled {
		return ErrPoolFull
	}
	if nodeType != "LOCAL" {
		return nil
	}

	used, err := NodeUsedBytes(ctx, tx, nodeID)
	if err != nil {
		return err
	}

	if capacity <= 0 {
		capacity = 1000 * 1024 * 1024 * 1024
	}
	reserve := capacity / 20
	if capacity%20 != 0 {
		reserve++
	}
	free := capacity - reserve - used

	if driver != nil && nodePath != "" {
		total, physicalFree, statErr := driver.StatLocal(ctx, nodePath)
		if statErr == nil && total > 0 {
			physicalReserve := total / 20
			if total%20 != 0 {
				physicalReserve++
			}
			if physicalFree <= physicalReserve {
				return ErrPoolFull
			}
		}
	}

	if free < additionalBytes {
		return ErrPoolFull
	}
	return nil
}
