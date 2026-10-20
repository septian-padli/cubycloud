package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"github.com/cubycloud/backend/internal/database"
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

// Login authenticates user with identifier (username or email) and password
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*database.User, error) {
	if req.Identifier == "" || req.Password == "" {
		return nil, echo.NewHTTPError(400, "username/email and password are required")
	}

	user, err := s.repo.GetUserByIdentifier(ctx, req.Identifier)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, echo.NewHTTPError(401, "invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, echo.NewHTTPError(401, "invalid username or password")
	}

	return user, nil
}

// GetUserByID retrieves user by ID
func (s *Service) GetUserByID(ctx context.Context, id string) (*database.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// GetUserUsage calculates total deduplicated file size for user
func (s *Service) GetUserUsage(ctx context.Context, userID string) (int64, error) {
	var usedBytes int64
	query := `
SELECT COALESCE(SUM(size_bytes), 0)
FROM (
    SELECT COALESCE(disk_key, id) AS k, MAX(size_bytes) AS size_bytes
    FROM my_storage_items
    WHERE owner_id = ? AND kind = 'file'
    GROUP BY k
)`
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&usedBytes)
	if err != nil {
		return 0, err
	}
	return usedBytes, nil
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
	if user.RecoveryKeyHash == nil || *user.RecoveryKeyHash == "" {
		return nil, echo.NewHTTPError(400, "recovery key not set for this user")
	}

	oldHash := *user.RecoveryKeyHash
	canonicalKey, valid := normalizeRecoveryKey(req.RecoveryKey)
	if !valid {
		return nil, echo.NewHTTPError(401, "invalid recovery key")
	}
	if len(canonicalKey) == 64 {
		if !strings.HasPrefix(oldHash, "$2") || bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(canonicalKey)) != nil {
			return nil, echo.NewHTTPError(401, "invalid recovery key")
		}
	} else {
		keyHash := hashRecoveryKey(canonicalKey)
		if subtle.ConstantTimeCompare([]byte(oldHash), []byte(keyHash)) != 1 {
			return nil, echo.NewHTTPError(401, "invalid recovery key")
		}
	}

	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	newRecoveryKey, err := generateRecoveryKey()
	if err != nil {
		return nil, fmt.Errorf("generate recovery key: %w", err)
	}
	newRecoveryKeyHash := hashRecoveryKey(strings.ReplaceAll(newRecoveryKey, "-", ""))

	// The old hash predicate makes recovery-key consumption atomic: concurrent
	// reset requests can only replace the same key once.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	updated, err := s.repo.UpdatePasswordAndRecoveryKeyTx(
		ctx, tx, user.ID, oldHash, string(newPasswordHash), newRecoveryKeyHash,
	)
	if err != nil {
		return nil, fmt.Errorf("consume recovery key: %w", err)
	}
	if !updated {
		return nil, echo.NewHTTPError(401, "invalid recovery key")
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &ResetPasswordResponse{
		Message:        "password reset successful",
		NewRecoveryKey: newRecoveryKey,
	}, nil
}

func normalizeRecoveryKey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 64 {
		if _, err := hex.DecodeString(value); err == nil {
			return strings.ToLower(value), true
		}
	}

	var normalized strings.Builder
	for _, char := range value {
		if char == '-' || unicode.IsSpace(char) {
			continue
		}
		char = unicode.ToUpper(char)
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return "", false
		}
		normalized.WriteRune(char)
	}
	key := normalized.String()
	return key, len(key) == 30
}

func hashRecoveryKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func generateRecoveryKey() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	random := make([]byte, 30)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	var key strings.Builder
	key.Grow(35)
	for i, value := range random {
		if i > 0 && i%5 == 0 {
			key.WriteByte('-')
		}
		key.WriteByte(alphabet[value&31])
	}
	return key.String(), nil
}
