package admin

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/cubycloud/backend/internal/database"
	"github.com/uptrace/bun"
)

type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

// GetSetting retrieves a setting by key
func (r *Repository) GetSetting(ctx context.Context, key string) (string, error) {
	var setting database.Setting
	err := r.db.NewSelect().
		Model(&setting).
		Where("key = ?", key).
		Scan(ctx)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// SetSetting updates or inserts a setting
func (r *Repository) SetSetting(ctx context.Context, key, value string) error {
	setting := &database.Setting{
		Key:   key,
		Value: value,
	}
	_, err := r.db.NewInsert().
		Model(setting).
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Exec(ctx)
	return err
}

// Checkpoint runs WAL checkpoint truncate
func (r *Repository) Checkpoint(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// BackupTo writes a consistent database snapshot to a new file.
func (r *Repository) BackupTo(ctx context.Context, destPath string) error {
	_, err := r.db.ExecContext(ctx, "VACUUM INTO '"+strings.ReplaceAll(destPath, "'", "''")+"'")
	return err
}

// GetAllStorageNodes retrieves all storage nodes
func (r *Repository) GetAllStorageNodes(ctx context.Context) ([]database.StorageNode, error) {
	var nodes []database.StorageNode
	err := r.db.NewSelect().Model(&nodes).Order("created_at ASC").Order("id ASC").Scan(ctx)
	return nodes, err
}

// GetStorageNodeByID retrieves a storage node by ID
func (r *Repository) GetStorageNodeByID(ctx context.Context, id string) (*database.StorageNode, error) {
	var node database.StorageNode
	err := r.db.NewSelect().
		Model(&node).
		Where("id = ?", id).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// DisableNode sets enabled=false for a node
func (r *Repository) DisableNode(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().
		Model((*database.StorageNode)(nil)).
		Set("enabled = ?", false).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// UpsertStorageNode inserts or updates a storage node
func (r *Repository) UpsertStorageNode(ctx context.Context, node *database.StorageNode) error {
	_, err := r.db.NewInsert().
		Model(node).
		On("CONFLICT (id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("capacity_bytes = EXCLUDED.capacity_bytes").
		Set("enabled = EXCLUDED.enabled").
		Exec(ctx)
	return err
}

// GetNodeByPath retrieves node by path
func (r *Repository) GetNodeByPath(ctx context.Context, path string) (*database.StorageNode, error) {
	var node database.StorageNode
	err := r.db.NewSelect().
		Model(&node).
		Where("path = ?", path).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &node, err
}

// GetNodeByName retrieves cloud node by bucket name
func (r *Repository) GetNodeByName(ctx context.Context, bucket string) (*database.StorageNode, error) {
	var node database.StorageNode
	err := r.db.NewSelect().
		Model(&node).
		Where("bucket = ? AND type = 'S3'", bucket).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &node, err
}

// CreateNode inserts a new storage node
func (r *Repository) CreateNode(ctx context.Context, node *database.StorageNode) error {
	_, err := r.db.NewInsert().
		Model(node).
		Exec(ctx)
	return err
}

// UpdateNode updates a storage node
func (r *Repository) UpdateNode(ctx context.Context, node *database.StorageNode) error {
	_, err := r.db.NewUpdate().
		Model(node).
		WherePK().
		Exec(ctx)
	return err
}
func (r *Repository) HasStorageNodeName(ctx context.Context, name, excludeID string) (bool, error) {
	nodes, err := r.GetAllStorageNodes(ctx)
	if err != nil {
		return false, err
	}
	for _, node := range nodes {
		if node.ID == excludeID {
			continue
		}
		nodeName := node.Name
		if nodeName == "" {
			if node.Type == "LOCAL" && node.Path != nil {
				nodeName = filepath.Base(*node.Path)
			} else if node.Bucket != nil {
				nodeName = *node.Bucket
			}
		}
		if strings.EqualFold(strings.TrimSpace(nodeName), strings.TrimSpace(name)) {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) StorageNodeUsedBytes(ctx context.Context, nodeID, legacyNodeID string) (int64, error) {
	var used int64
	err := r.db.NewRaw(`
		SELECT COALESCE(SUM(size_bytes), 0) FROM (
			SELECT disk_key, MAX(size_bytes) AS size_bytes
			FROM (
				SELECT disk_key, size_bytes
				FROM my_storage_items
				WHERE kind = 'file' AND disk_key IS NOT NULL
					AND (storage_node_id = ? OR (storage_node_id IS NULL AND ? = ?))
				UNION ALL
				SELECT disk_key, size_bytes
				FROM organization_items
				WHERE kind = 'file' AND disk_key IS NOT NULL
					AND (storage_node_id = ? OR (storage_node_id IS NULL AND ? = ?))
			)
			GROUP BY disk_key
		)`, nodeID, nodeID, legacyNodeID, nodeID, nodeID, legacyNodeID).Scan(ctx, &used)
	return used, err
}
