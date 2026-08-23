package auth

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

type Service struct {
	db   *bun.DB
	repo *Repository
}

func NewService(db *bun.DB) *Service {
	return &Service{
		db:   db,
		repo: NewRepository(db),
	}
}

// ResetPassword resets user password using recovery key
func (s *Service) ResetPassword(ctx context.Context, req *ResetPasswordRequest) (*ResetPasswordResponse, error) {
	// Get user by username
	user, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, echo.NewHTTPError(404, "user not found")
	}

	// Check if user has recovery key
	if user.RecoveryKeyHash == nil || *user.RecoveryKeyHash == "" {
		return nil, echo.NewHTTPError(400, "recovery key not set for this user")
	}

	// Verify recovery key
	if err := bcrypt.CompareHashAndPassword([]byte(*user.RecoveryKeyHash), []byte(req.RecoveryKey)); err != nil {
		return nil, echo.NewHTTPError(401, "invalid recovery key")
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Update password in transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := s.repo.UpdatePasswordTx(ctx, tx, user.ID, string(newPasswordHash)); err != nil {
		return nil, fmt.Errorf("update password: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &ResetPasswordResponse{
		Message: "password reset successful",
	}, nil
}
