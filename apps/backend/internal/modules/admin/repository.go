package admin

import (
	"context"
	"database/sql"

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

// GetAllStorageNodes retrieves all storage nodes
func (r *Repository) GetAllStorageNodes(ctx context.Context) ([]database.StorageNode, error) {
	var nodes []database.StorageNode
	err := r.db.NewSelect().
		Model(&nodes).
		Order("created_at ASC").
		Scan(ctx)
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
		Set("capacity_bytes = EXCLUDED.capacity_bytes").
		Set("enabled = EXCLUDED.enabled").
		Exec(ctx)
	return err
}
