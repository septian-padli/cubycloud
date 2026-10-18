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

// GetUserByIdentifier retrieves user by username or email
func (r *Repository) GetUserByIdentifier(ctx context.Context, identifier string) (*database.User, error) {
	var user database.User
	err := r.db.NewSelect().
		Model(&user).
		Where("username = ? OR email = ?", identifier, identifier).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves user by primary key ID
func (r *Repository) GetUserByID(ctx context.Context, id string) (*database.User, error) {
	var user database.User
	err := r.db.NewSelect().
		Model(&user).
		Where("id = ?", id).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdatePasswordAndRecoveryKeyTx atomically replaces the password and consumes the supplied recovery hash.
func (r *Repository) UpdatePasswordAndRecoveryKeyTx(
	ctx context.Context,
	tx bun.Tx,
	userID, oldRecoveryHash, passwordHash, newRecoveryHash string,
) (bool, error) {
	result, err := tx.NewUpdate().
		Model((*database.User)(nil)).
		Set("password_hash = ?", passwordHash).
		Set("recovery_key_hash = ?", newRecoveryHash).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND recovery_key_hash = ?", userID, oldRecoveryHash).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
