package setup

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

type Service struct {
	repo         *Repository
	db           *bun.DB
	storage      storage.StorageDriver
	setupGuard   *middleware.SetupGuard
	dataDirPath  string
}

func NewService(db *bun.DB, storage storage.StorageDriver, setupGuard *middleware.SetupGuard, dataDirPath string) *Service {
	return &Service{
		repo:        NewRepository(db),
		db:          db,
		storage:     storage,
		setupGuard:  setupGuard,
		dataDirPath: dataDirPath,
	}
}

func (s *Service) ScanStorageNodes(ctx context.Context) (*ScanResponse, error) {
	storageRoot := filepath.Join(s.dataDirPath, "storage")
	
	entries, err := os.ReadDir(storageRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// Storage directory belum ada, gunakan DATA_DIR sebagai default node
			capacity, free, err := s.storage.StatLocal(ctx, s.dataDirPath)
			if err != nil {
				return &ScanResponse{Nodes: []StorageNodeScan{}, Devices: []interface{}{}}, nil
			}
			return &ScanResponse{
				Nodes: []StorageNodeScan{{
					Path:          filepath.Join(s.dataDirPath, "storage", "default"),
					CapacityBytes: capacity,
					FreeBytes:     free,
				}},
				Devices: []interface{}{},
			}, nil
		}
		return nil, fmt.Errorf("read storage dir: %w", err)
	}

	var nodes []StorageNodeScan
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		nodePath := filepath.Join(storageRoot, entry.Name())
		capacity, free, err := s.storage.StatLocal(ctx, nodePath)
		if err != nil {
			continue // ponytail: skip unreadable nodes
		}
		
		nodes = append(nodes, StorageNodeScan{
			Path:          nodePath,
			CapacityBytes: capacity,
			FreeBytes:     free,
		})
	}

	// Kalau storage/ kosong, gunakan DATA_DIR sebagai default
	if len(nodes) == 0 {
		capacity, free, err := s.storage.StatLocal(ctx, s.dataDirPath)
		if err == nil {
			nodes = append(nodes, StorageNodeScan{
				Path:          filepath.Join(s.dataDirPath, "storage", "default"),
				CapacityBytes: capacity,
				FreeBytes:     free,
			})
		}
	}

	return &ScanResponse{
		Nodes:   nodes,
		Devices: []interface{}{},
	}, nil
}

func (s *Service) GetSetupStatus(ctx context.Context) (*StatusResponse, error) {
	completed := s.setupGuard.IsCompleted()
	if !completed {
		dbCompleted, err := s.repo.IsSetupCompleted(ctx)
		if err == nil && dbCompleted {
			s.setupGuard.MarkCompleted()
			completed = true
		}
	}
	return &StatusResponse{
		IsSetupCompleted: completed,
	}, nil
}

func (s *Service) CompleteSetup(ctx context.Context, req *CompleteSetupRequest) (*CompleteSetupResponse, error) {
	// Guard: check setup already completed
	if s.setupGuard.IsCompleted() {
		return nil, echo.NewHTTPError(http.StatusForbidden, "setup already completed")
	}

	completed, err := s.repo.IsSetupCompleted(ctx)
	if err != nil {
		return nil, fmt.Errorf("check setup status: %w", err)
	}
	if completed {
		s.setupGuard.MarkCompleted()
		return nil, echo.NewHTTPError(http.StatusForbidden, "setup already completed")
	}

	// Validate
	if err := s.validate(ctx, req); err != nil {
		return nil, err
	}

	// Use client-provided recovery key
	recoveryKey := req.Admin.RecoveryKey
	if len(recoveryKey) != 64 {
		return nil, echo.NewHTTPError(400, "recovery key must be 64 hex characters")
	}
	// Validate hex
	if _, err := hex.DecodeString(recoveryKey); err != nil {
		return nil, echo.NewHTTPError(400, "recovery key must be valid hex string")
	}

	// Hash password and recovery key
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Admin.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	recoveryKeyHash, err := bcrypt.GenerateFromPassword([]byte(recoveryKey), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash recovery key: %w", err)
	}

	// Transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Create user
	userID := ulid.Make().String()
	recoveryKeyHashStr := string(recoveryKeyHash)
	user := &database.User{
		ID:              userID,
		Username:        req.Admin.Username,
		Email:           req.Admin.Email,
		PasswordHash:    string(passwordHash),
		RecoveryKeyHash: &recoveryKeyHashStr,
		Role:            "SUPER_ADMIN",
		QuotaBytes:      -1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.repo.CreateUserTx(ctx, tx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Create storage nodes (enabled only)
	var nodes []database.StorageNode
	keyMaterial := []byte(s.dataDirPath)

	for _, n := range req.StorageNodes {
		if !n.Enabled {
			continue
		}
		nodeID := ulid.Make().String()
		node := database.StorageNode{
			ID:            nodeID,
			Type:          "LOCAL",
			Path:          &n.Path,
			CapacityBytes: n.CapacityBytes,
			Enabled:       true,
			CreatedAt:     time.Now(),
		}
		nodes = append(nodes, node)
	}

	// Add cloud node if enabled
	if req.Cloud.Enabled {
		nodeID := ulid.Make().String()
		endpoint := req.Cloud.Endpoint
		bucket := req.Cloud.Bucket
		region := req.Cloud.Region

		accessKeyEnc, err := storage.EncryptCredential([]byte(req.Cloud.AccessKey), keyMaterial)
		if err != nil {
			return nil, fmt.Errorf("encrypt access key: %w", err)
		}

		secretKeyEnc, err := storage.EncryptCredential([]byte(req.Cloud.SecretKey), keyMaterial)
		if err != nil {
			return nil, fmt.Errorf("encrypt secret key: %w", err)
		}

		node := database.StorageNode{
			ID:                 nodeID,
			Type:               "S3",
			Endpoint:           &endpoint,
			Bucket:             &bucket,
			Region:             &region,
			AccessKeyEncrypted: accessKeyEnc,
			SecretKeyEncrypted: secretKeyEnc,
			CapacityBytes:      0, // Cloud: no fixed capacity
			Enabled:            true,
			CreatedAt:          time.Now(),
		}
		nodes = append(nodes, node)
	}

	if err := s.repo.CreateStorageNodesTx(ctx, tx, nodes); err != nil {
		return nil, fmt.Errorf("create storage nodes: %w", err)
	}

	// Update settings
	settings := map[string]string{
		"is_setup_completed":         "true",
		"placement_strategy":         req.PlacementStrategy,
		"jwt_secret_version":         "1",
		"default_user_quota_gb":      "100",
		"worker_concurrency_limit":   "2",
		"audit_log_retention_days":   "90",
	}
	if err := s.repo.UpdateSettingsTx(ctx, tx, settings); err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	// Update in-memory guard
	s.setupGuard.MarkCompleted()

	return &CompleteSetupResponse{
		RecoveryKey: recoveryKey,
		Redirect:    "/drive/home",
	}, nil
}

func (s *Service) validate(ctx context.Context, req *CompleteSetupRequest) error {
	// Username unique
	exists, err := s.repo.UsernameExists(ctx, req.Admin.Username)
	if err != nil {
		return fmt.Errorf("check username: %w", err)
	}
	if exists {
		return echo.NewHTTPError(400, "username already exists")
	}

	// Email unique
	exists, err = s.repo.EmailExists(ctx, req.Admin.Email)
	if err != nil {
		return fmt.Errorf("check email: %w", err)
	}
	if exists {
		return echo.NewHTTPError(400, "email already exists")
	}

	// Password length
	if len(req.Admin.Password) < 8 {
		return echo.NewHTTPError(400, "password must be at least 8 characters")
	}

	// At least one enabled node
	hasEnabled := false
	for _, n := range req.StorageNodes {
		if n.Enabled {
			hasEnabled = true
			break
		}
	}
	if !hasEnabled && !req.Cloud.Enabled {
		return echo.NewHTTPError(400, "at least one storage node must be enabled")
	}

	// Validate local nodes
	for _, n := range req.StorageNodes {
		if !n.Enabled {
			continue
		}

		// Auto-create path if not exists
		if _, err := os.Stat(n.Path); os.IsNotExist(err) {
			if err := os.MkdirAll(n.Path, 0755); err != nil {
				return echo.NewHTTPError(400, fmt.Sprintf("cannot create storage directory: %s", n.Path))
			}
		}

		// Writable test
		testFile := filepath.Join(n.Path, ".cuby_write_test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			return echo.NewHTTPError(400, fmt.Sprintf("local node not writable: %s", n.Path))
		}
		os.Remove(testFile)
	}

	// Validate cloud credentials
	if req.Cloud.Enabled {
		if err := s.storage.ValidateCloudCredentials(
			ctx,
			req.Cloud.Endpoint,
			req.Cloud.Region,
			req.Cloud.Bucket,
			req.Cloud.AccessKey,
			req.Cloud.SecretKey,
		); err != nil {
			return echo.NewHTTPError(400, fmt.Sprintf("cloud credentials invalid: %v", err))
		}
	}

	return nil
}
