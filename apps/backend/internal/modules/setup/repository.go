package setup

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

func (r *Repository) IsSetupCompleted(ctx context.Context) (bool, error) {
	var setting database.Setting
	err := r.db.NewSelect().Model(&setting).Where("key = ?", "is_setup_completed").Scan(ctx)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return setting.Value == "true", nil
}

func (r *Repository) UsernameExists(ctx context.Context, username string) (bool, error) {
	count, err := r.db.NewSelect().Model((*database.User)(nil)).Where("username = ?", username).Count(ctx)
	return count > 0, err
}

func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	count, err := r.db.NewSelect().Model((*database.User)(nil)).Where("email = ?", email).Count(ctx)
	return count > 0, err
}

func (r *Repository) CreateUserTx(ctx context.Context, tx bun.Tx, user *database.User) error {
	_, err := tx.NewInsert().Model(user).Exec(ctx)
	return err
}

func (r *Repository) CreateStorageNodesTx(ctx context.Context, tx bun.Tx, nodes []database.StorageNode) error {
	if len(nodes) == 0 {
		return nil
	}
	_, err := tx.NewInsert().Model(&nodes).Exec(ctx)
	return err
}

func (r *Repository) UpdateSettingsTx(ctx context.Context, tx bun.Tx, kvs map[string]string) error {
	for k, v := range kvs {
		setting := &database.Setting{Key: k, Value: v}
		_, err := tx.NewInsert().Model(setting).
			On("CONFLICT (key) DO UPDATE").
			Set("value = EXCLUDED.value").
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
