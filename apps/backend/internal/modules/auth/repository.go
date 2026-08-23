package auth

import (
	"context"
	"database/sql"
	"time"

	"github.com/cubycloud/backend/internal/database"

	"github.com/uptrace/bun"
)
type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

// GetUserByUsername retrieves user by username
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	var user database.User
	err := r.db.NewSelect().
		Model(&user).
		Where("username = ?", username).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdatePasswordTx updates user password in transaction
func (r *Repository) UpdatePasswordTx(ctx context.Context, tx bun.Tx, userID string, passwordHash string) error {
	_, err := tx.NewUpdate().
		Model((*database.User)(nil)).
		Set("password_hash = ?", passwordHash).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}
