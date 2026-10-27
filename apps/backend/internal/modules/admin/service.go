package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

const (
	SettingDefaultQuotaGB     = "default_user_quota_gb"
	SettingAuditRetentionDays = "audit_log_retention_days"
	SettingWorkerConcurrency  = "worker_concurrency_limit"
	SettingPlacementStrategy  = "placement_strategy"
)

type ConcurrencyController interface {
	SetConcurrency(limit int)
}

type Service struct {
	repo           *Repository
	db             *bun.DB
	storage        storage.StorageDriver
	dataDirPath    string
	queue          *jobs.Queue
	concurrencyCtl ConcurrencyController
	nodeNameMu     sync.Mutex
}
func NewService(db *bun.DB, storage storage.StorageDriver, dataDirPath string, queue ...*jobs.Queue) *Service {
	var q *jobs.Queue
	if len(queue) > 0 && queue[0] != nil {
		q = queue[0]
	} else {
		q = jobs.NewQueue(db)
	}
	return &Service{
		repo:        NewRepository(db),
		db:          db,
		storage:     storage,
		dataDirPath: dataDirPath,
		queue:       q,
	}
}

func (s *Service) SetConcurrencyController(ctl ConcurrencyController) {
	s.concurrencyCtl = ctl
}

// GetSystemSettings retrieves all system settings
func (s *Service) GetSystemSettings(ctx context.Context) (*SystemSettingsResponse, error) {
	quotaGB, err := s.repo.GetSetting(ctx, SettingDefaultQuotaGB)
	if err != nil {
		quotaGB = "-1"
	}

	retentionDays, err := s.repo.GetSetting(ctx, SettingAuditRetentionDays)
	if err != nil {
		retentionDays = "90"
	}

	concurrency, err := s.repo.GetSetting(ctx, SettingWorkerConcurrency)
	if err != nil {
		concurrency = "2"
	}

	strategy, err := s.repo.GetSetting(ctx, SettingPlacementStrategy)
	if err != nil {
		strategy = "FILL_FIRST"
	}

	quotaValue, _ := strconv.ParseInt(quotaGB, 10, 64)
	retentionValue, _ := strconv.Atoi(retentionDays)
	if retentionValue == 0 {
		retentionValue = 90
	}
	concurrencyValue, _ := strconv.Atoi(concurrency)
	if concurrencyValue == 0 {
		concurrencyValue = 2
	}

	quota := QuotaPolicy{
		Mode: "limited",
		GB:   int(quotaValue),
	}
	if quotaValue <= 0 {
		quota.Mode = "unlimited"
		quota.GB = 0
	}

	return &SystemSettingsResponse{
		DefaultQuota:       quota,
		AuditRetentionDays: retentionValue,
		WorkerConcurrency:  concurrencyValue,
		PlacementStrategy:  strategy,
	}, nil
}

// Checkpoint checkpoints the WAL journal to ensure data is flushed
func (s *Service) Checkpoint(ctx context.Context) error {
	return s.repo.Checkpoint(ctx)
}

// BackupTo writes a consistent database snapshot to a new file.
func (s *Service) BackupTo(ctx context.Context, destPath string) error {
	return s.repo.BackupTo(ctx, destPath)
}

// UpdateSystemSettings updates system settings with validation
func (s *Service) UpdateSystemSettings(ctx context.Context, req *UpdateSettingsRequest) error {
	if req.DefaultQuota != nil {
		if req.DefaultQuota.Mode == "limited" {
			if req.DefaultQuota.GB < 1 {
				return fmt.Errorf("default quota must be at least 1 GB")
			}
			if err := s.repo.SetSetting(ctx, SettingDefaultQuotaGB, strconv.Itoa(req.DefaultQuota.GB)); err != nil {
				return fmt.Errorf("set default quota: %w", err)
			}
		} else if req.DefaultQuota.Mode == "unlimited" {
			if err := s.repo.SetSetting(ctx, SettingDefaultQuotaGB, "-1"); err != nil {
				return fmt.Errorf("set unlimited quota: %w", err)
			}
		} else {
			return fmt.Errorf("invalid quota mode: %s", req.DefaultQuota.Mode)
		}
	}

	if req.AuditRetentionDays != nil {
		days := *req.AuditRetentionDays
		validDays := map[int]bool{7: true, 14: true, 30: true, 60: true, 90: true}
		if !validDays[days] {
			return fmt.Errorf("audit retention must be one of: 7, 14, 30, 60, 90")
		}
		if err := s.repo.SetSetting(ctx, SettingAuditRetentionDays, strconv.Itoa(days)); err != nil {
			return fmt.Errorf("set audit retention: %w", err)
		}
	}

	if req.WorkerConcurrency != nil {
		concurrency := *req.WorkerConcurrency
		if concurrency < 1 || concurrency > 10 {
			return fmt.Errorf("worker concurrency must be between 1 and 10")
		}
		if err := s.repo.SetSetting(ctx, SettingWorkerConcurrency, strconv.Itoa(concurrency)); err != nil {
			return fmt.Errorf("set worker concurrency: %w", err)
		}
		if s.concurrencyCtl != nil {
			s.concurrencyCtl.SetConcurrency(concurrency)
		}
	}
	if req.PlacementStrategy != nil {
		strategy := *req.PlacementStrategy
		if strategy != "FILL_FIRST" && strategy != "ROUND_ROBIN" && strategy != "MOST_FREE_SPACE" {
			return fmt.Errorf("invalid placement strategy: %s", strategy)
		}
		if err := s.repo.SetSetting(ctx, SettingPlacementStrategy, strategy); err != nil {
			return fmt.Errorf("set placement strategy: %w", err)
		}
	}

	return nil
}

func (s *Service) legacyLocalNodeID(ctx context.Context) (string, error) {
	nodes, err := s.repo.GetAllStorageNodes(ctx)
	if err != nil {
		return "", err
	}
	first := ""
	defaultPath := filepath.Join(s.dataDirPath, "storage", "default")
	for _, node := range nodes {
		if node.Type != "LOCAL" {
			continue
		}
		if first == "" {
			first = node.ID
		}
		if node.Path != nil && *node.Path == defaultPath {
			return node.ID, nil
		}
	}
	return first, nil
}
func (s *Service) uniqueDiscoveredNodeName(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		base = "Local storage"
	}
	for suffix := 1; ; suffix++ {
		name := base
		if suffix > 1 {
			name = fmt.Sprintf("%s (%d)", base, suffix)
		}
		exists, err := s.repo.HasStorageNodeName(ctx, name, "")
		if err != nil {
			return "", err
		}
		if !exists {
			return name, nil
		}
	}
}

// GetStorageNodes lists all storage nodes with capacity info
func (s *Service) GetStorageNodes(ctx context.Context) ([]StorageNodeResponse, error) {
	nodes, err := s.repo.GetAllStorageNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get storage nodes: %w", err)
	}

	var response []StorageNodeResponse
	legacyNodeID, err := s.legacyLocalNodeID(ctx)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		used, err := s.repo.StorageNodeUsedBytes(ctx, node.ID, legacyNodeID)
		if err != nil {
			return nil, fmt.Errorf("get storage node usage: %w", err)
		}
		capGB := node.CapacityBytes / (1024 * 1024 * 1024)
		freeBytes := node.CapacityBytes - used
		if freeBytes < 0 {
			freeBytes = 0
		}
		nodeResp := StorageNodeResponse{
			ID: node.ID, Type: node.Type, Name: node.Name, CapacityCapGB: capGB,
			TotalGB: capGB, UsedBytes: used, Enabled: node.Enabled,
			FreeGB: freeBytes / (1024 * 1024 * 1024),
			SafeToDisconnect: !node.Enabled && used == 0,
		}
		var migrationJob database.Job
		if err := s.db.NewSelect().Model(&migrationJob).
			Where("type = ?", jobs.TypeNodeMigration).
			Where("json_extract(payload, '$.source_node_id') = ?", node.ID).
			Order("created_at DESC").
			Limit(1).
			Scan(ctx); err == nil {
			nodeResp.MigrationJobID = migrationJob.ID
			switch migrationJob.Status {
			case jobs.StatusPending, jobs.StatusProcessing:
				nodeResp.MigrationStatus = "MIGRATING"
				nodeResp.SafeToDisconnect = false
			case jobs.StatusFailed:
				nodeResp.MigrationStatus = "FAILED"
				nodeResp.SafeToDisconnect = false
			case jobs.StatusCompleted:
				if used == 0 {
					nodeResp.MigrationStatus = "COMPLETED"
					nodeResp.SafeToDisconnect = true
				} else {
					nodeResp.MigrationStatus = "FAILED"
					nodeResp.SafeToDisconnect = false
				}
			}
		}
		if node.Type == "LOCAL" && node.Path != nil {
			nodeResp.Path = *node.Path
			nodeResp.Kind = storage.DetectDiskKind(ctx, *node.Path)
			if nodeResp.Name == "" {
				nodeResp.Name = filepath.Base(*node.Path)
			}
		} else if node.Type == "S3" {
			nodeResp.Path = ""
			nodeResp.Kind = "S3"
			if node.Endpoint != nil {
				nodeResp.Endpoint = *node.Endpoint
			}
			if node.Bucket != nil {
				nodeResp.Bucket = *node.Bucket
			}
			if node.Region != nil {
				nodeResp.Region = *node.Region
			}
		}
		response = append(response, nodeResp)
	}
	return response, nil
}

// DisableStorageNode disables a storage node
func (s *Service) DisableStorageNode(ctx context.Context, nodeID string) error {
	node, err := s.repo.GetStorageNodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	if !node.Enabled {
		return fmt.Errorf("node already disabled")
	}

	if err := s.repo.DisableNode(ctx, nodeID); err != nil {
		return fmt.Errorf("disable node: %w", err)
	}

	return nil
}

// DetachStorageNode disables a storage node and enqueues a physical migration background job if files exist
func (s *Service) DetachStorageNode(ctx context.Context, nodeID, targetNodeID, adminUserID string) (*DetachNodeResponse, error) {
	node, err := s.repo.GetStorageNodeByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("node not found")
	}

	// Immediately disable node to halt any new uploads or allocations
	if node.Enabled {
		if err := s.repo.DisableNode(ctx, nodeID); err != nil {
			return nil, fmt.Errorf("disable node: %w", err)
		}
	}

	legacyNodeID, err := s.legacyLocalNodeID(ctx)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.StorageNodeUsedBytes(ctx, node.ID, legacyNodeID)
	if err != nil {
		return nil, fmt.Errorf("check node usage: %w", err)
	}

	if used == 0 {
		return &DetachNodeResponse{
			Message:          "node detached successfully (no files to migrate)",
			SafeToDisconnect: true,
		}, nil
	}

	if s.queue == nil {
		s.queue = jobs.NewQueue(s.db)
	}

	payload := jobs.NodeMigrationPayload{
		SourceNodeID: nodeID,
		TargetNodeID: targetNodeID,
	}

	enqueuedJob, err := s.queue.Enqueue(ctx, jobs.TypeNodeMigration, adminUserID, payload)
	if err != nil {
		return nil, fmt.Errorf("enqueue migration job: %w", err)
	}

	return &DetachNodeResponse{
		Message:          "node detachment and migration started",
		JobID:            &enqueuedJob.ID,
		SafeToDisconnect: false,
	}, nil
}

// RescanStorageNodes rescans local storage nodes
func (s *Service) RescanStorageNodes(ctx context.Context) ([]StorageNodeResponse, error) {
	s.nodeNameMu.Lock()
	defer s.nodeNameMu.Unlock()
	storageRoot := filepath.Join(s.dataDirPath, "storage")

	entries, err := os.ReadDir(storageRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// No storage directory, create default node
			defaultPath := filepath.Join(s.dataDirPath, "storage", "default")
			capacity, _, err := s.storage.StatLocal(ctx, s.dataDirPath)
			if err != nil {
				return nil, fmt.Errorf("stat default path: %w", err)
			}

			name, err := s.uniqueDiscoveredNodeName(ctx, filepath.Base(defaultPath))
			if err != nil {
				return nil, err
			}
			node := &database.StorageNode{
				ID: ulid.Make().String(), Name: name, Type: "LOCAL",
				Path: &defaultPath, CapacityBytes: capacity, Enabled: true,
			}

			if err := s.repo.UpsertStorageNode(ctx, node); err != nil {
				return nil, fmt.Errorf("upsert default node: %w", err)
			}

			return s.GetStorageNodes(ctx)
		}
		return nil, fmt.Errorf("read storage dir: %w", err)
	}

	// Scan existing directories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		nodePath := filepath.Join(storageRoot, entry.Name())
		capacity, _, err := s.storage.StatLocal(ctx, nodePath)
		if err != nil {
			continue // ponytail: skip unreadable nodes
		}

		// Check if node exists
		existingNodes, _ := s.repo.GetAllStorageNodes(ctx)
		var existingNode *database.StorageNode
		for i := range existingNodes {
			if existingNodes[i].Path != nil && *existingNodes[i].Path == nodePath {
				existingNode = &existingNodes[i]
				break
			}
		}

		if existingNode != nil {
			if existingNode.CapacityBytes <= 0 {
				existingNode.CapacityBytes = capacity
			}
			if err := s.repo.UpsertStorageNode(ctx, existingNode); err != nil {
				continue
			}
		} else {
			// Create new node
			name, err := s.uniqueDiscoveredNodeName(ctx, entry.Name())
			if err != nil {
				continue
			}
			node := &database.StorageNode{
				ID: ulid.Make().String(), Name: name, Type: "LOCAL",
				Path: &nodePath, CapacityBytes: capacity, Enabled: true,
			}
			if err := s.repo.UpsertStorageNode(ctx, node); err != nil {
				continue
			}
		}
	}

	return s.GetStorageNodes(ctx)
}

// CreateLocalNode adds a new local disk storage node
func (s *Service) CreateLocalNode(ctx context.Context, req *CreateLocalNodeRequest) (*StorageNodeResponse, error) {
	s.nodeNameMu.Lock()
	defer s.nodeNameMu.Unlock()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	exists, err := s.repo.HasStorageNodeName(ctx, name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("storage node name already exists")
	}
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", req.Path)
	}
	existing, err := s.repo.GetNodeByPath(ctx, req.Path)
	if err != nil {
		return nil, fmt.Errorf("check existing node: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("node with path %s already exists", req.Path)
	}
	node := &database.StorageNode{
		ID: ulid.Make().String(), Name: name, Type: "LOCAL", Path: &req.Path,
		CapacityBytes: req.CapacityCapGB * 1024 * 1024 * 1024, Enabled: req.Enabled,
	}
	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	nodes, err := s.GetStorageNodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, result := range nodes {
		if result.ID == node.ID {
			return &result, nil
		}
	}
	return nil, fmt.Errorf("created node not found")
}

// CreateCloudNode adds a new S3-compatible cloud storage node
func (s *Service) CreateCloudNode(ctx context.Context, req *CreateCloudNodeRequest) (*StorageNodeResponse, error) {
	s.nodeNameMu.Lock()
	defer s.nodeNameMu.Unlock()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	exists, err := s.repo.HasStorageNodeName(ctx, name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("storage node name already exists")
	}
	existing, err := s.repo.GetNodeByName(ctx, req.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check existing node: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("node with bucket %s already exists", req.Bucket)
	}

	if err := s.storage.ValidateCloudCredentials(ctx, req.Endpoint, req.Region, req.Bucket, req.AccessKey, req.SecretKey); err != nil {
		return nil, fmt.Errorf("validate cloud credentials: %w", err)
	}

	keyMaterial, err := storage.InitMasterKey(s.dataDirPath, os.Getenv("CUBYCLOUD_MASTER_KEY"), os.Getenv("CUBYCLOUD_MASTER_KEY_FILE"), s.db)
	if err != nil {
		return nil, fmt.Errorf("initialize credential encryption: %w", err)
	}
	accessEnc, err := storage.EncryptCredential([]byte(req.AccessKey), keyMaterial)
	if err != nil {
		return nil, errors.New("encrypt cloud credentials")
	}
	secretEnc, err := storage.EncryptCredential([]byte(req.SecretKey), keyMaterial)
	if err != nil {
		return nil, errors.New("encrypt cloud credentials")
	}

	node := &database.StorageNode{
		ID: ulid.Make().String(), Name: name, Type: "S3", Endpoint: &req.Endpoint,
		Bucket: &req.Bucket, Region: &req.Region, AccessKeyEncrypted: accessEnc,
		SecretKeyEncrypted: secretEnc,
		CapacityBytes:      req.CapacityCapGB * 1024 * 1024 * 1024, Enabled: req.Enabled,
	}
	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return &StorageNodeResponse{
		ID: node.ID, Type: "S3", Name: name, Kind: "S3", Endpoint: req.Endpoint, Bucket: req.Bucket, Region: req.Region,
		TotalGB: req.CapacityCapGB, FreeGB: req.CapacityCapGB, CapacityCapGB: req.CapacityCapGB, Enabled: req.Enabled,
	}, nil
}

// UpdateNode updates storage node capacity and/or enabled status.
func (s *Service) UpdateNode(ctx context.Context, nodeID string, req *UpdateNodeRequest) error {
	s.nodeNameMu.Lock()
	defer s.nodeNameMu.Unlock()
	node, err := s.repo.GetStorageNodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return fmt.Errorf("name is required")
		}
		exists, err := s.repo.HasStorageNodeName(ctx, name, nodeID)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("storage node name already exists")
		}
		node.Name = name
	}
	if req.CapacityCapGB != nil {
		if *req.CapacityCapGB < 1 {
			return fmt.Errorf("capacity must be at least 1 GB")
		}
		legacyNodeID, err := s.legacyLocalNodeID(ctx)
		if err != nil {
			return err
		}
		used, err := s.repo.StorageNodeUsedBytes(ctx, nodeID, legacyNodeID)
		if err != nil {
			return err
		}
		newCapacity := *req.CapacityCapGB * 1024 * 1024 * 1024
		if newCapacity < used {
			return fmt.Errorf("capacity cannot be below used bytes")
		}
		node.CapacityBytes = newCapacity
	}
	if req.Enabled != nil {
		node.Enabled = *req.Enabled
	}
	if err := s.repo.UpdateNode(ctx, node); err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	return nil
}

// DeleteNode soft-deletes a storage node (sets enabled=false)
func (s *Service) DeleteNode(ctx context.Context, nodeID string) error {
	node, err := s.repo.GetStorageNodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	// Soft delete by disabling
	node.Enabled = false
	if err := s.repo.UpdateNode(ctx, node); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}

	return nil
}

// GetDefaultQuotaBytes returns snapshot quota bytes for default user quota from settings
func (s *Service) GetDefaultQuotaBytes(ctx context.Context) (int64, error) {
	val, err := s.repo.GetSetting(ctx, SettingDefaultQuotaGB)
	if err != nil || val == "" {
		return -1, nil
	}
	quotaValue, err := strconv.ParseInt(val, 10, 64)
	if err != nil || quotaValue <= 0 {
		return -1, nil
	}
	return quotaValue * 1024 * 1024 * 1024, nil
}

// ListUsers lists all registered users with their quota and storage usage
func (s *Service) ListUsers(ctx context.Context) (*AdminUsersListResponse, error) {
 	rows, err := s.db.QueryContext(ctx, `SELECT id, username, email, role, status, COALESCE(quota_mode, 'DEFAULT'), quota_bytes, created_at FROM users WHERE deleted_at IS NULL ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	type rawUser struct {
		ID         string
		Username   string
		Email      string
		Role       string
		Status     string
		QuotaMode  string
		QuotaBytes int64
		CreatedAt  time.Time
	}
	var rawUsers []rawUser
	for rows.Next() {
		var u rawUser
 		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Status, &u.QuotaMode, &u.QuotaBytes, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		rawUsers = append(rawUsers, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	usageMap := make(map[string]int64)
	usageRows, err := s.db.QueryContext(ctx, `
		SELECT owner_id, COALESCE(SUM(size_bytes), 0)
		FROM (
			SELECT owner_id, disk_key, MAX(size_bytes) AS size_bytes
			FROM my_storage_items
			WHERE kind = 'file' AND disk_key IS NOT NULL
			GROUP BY owner_id, disk_key
		)
		GROUP BY owner_id
	`)
	if err == nil {
		defer usageRows.Close()
		for usageRows.Next() {
			var ownerID string
			var used int64
			if err := usageRows.Scan(&ownerID, &used); err == nil {
				usageMap[ownerID] = used
			}
		}
	}

	users := make([]AdminUserResponse, 0, len(rawUsers))
	for _, u := range rawUsers {
		usedBytes := usageMap[u.ID]
		usedGB := float64(usedBytes) / (1024.0 * 1024.0 * 1024.0)
		quotaMode := u.QuotaMode
		quotaBytes := u.QuotaBytes
		if u.Role == "SUPER_ADMIN" {
			quotaMode = "UNLIMITED"
			quotaBytes = -1
		}
		var quotaGB *float64
		if quotaBytes >= 0 {
			q := float64(quotaBytes) / (1024.0 * 1024.0 * 1024.0)
			quotaGB = &q
		}
		users = append(users, AdminUserResponse{
			Status:     u.Status,
			ID:         u.ID,
			Username:   u.Username,
			Email:      u.Email,
			Role:       u.Role,
			QuotaMode:  quotaMode,
			QuotaBytes: quotaBytes,
			QuotaGB:    quotaGB,
			UsedBytes:  usedBytes,
			UsedGB:     usedGB,
			CreatedAt:  u.CreatedAt.Format(time.RFC3339),
		})
	}

	return &AdminUsersListResponse{Users: users}, nil
}


// CreateUser creates a pending regular user and returns a one-time invitation credential.
func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) (*AdminUserResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" { return nil, fmt.Errorf("username is required") }
	email := strings.TrimSpace(req.Email)
	if email == "" { return nil, fmt.Errorf("email is required") }
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") { return nil, fmt.Errorf("invalid email format") }
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(username) = LOWER(?)`, username).Scan(&count); err != nil { return nil, fmt.Errorf("check username: %w", err) }
	if count > 0 { return nil, fmt.Errorf("username already exists") }
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER(?)`, email).Scan(&count); err != nil { return nil, fmt.Errorf("check email: %w", err) }
	if count > 0 { return nil, fmt.Errorf("email already exists") }
	mode := strings.ToUpper(strings.TrimSpace(req.QuotaMode))
	if mode == "" { mode = "DEFAULT" }
	var quotaBytes int64
	switch mode {
	case "DEFAULT":
		var err error
		quotaBytes, err = s.GetDefaultQuotaBytes(ctx)
		if err != nil { return nil, fmt.Errorf("get default quota: %w", err) }
	case "CUSTOM":
		if req.QuotaGB == nil || *req.QuotaGB < 0 { return nil, fmt.Errorf("quota_gb must be greater than or equal to 0") }
		quotaBytes = int64(math.Round(*req.QuotaGB * 1024 * 1024 * 1024))
	case "UNLIMITED": quotaBytes = -1
	default: return nil, fmt.Errorf("invalid quota mode: %s", req.QuotaMode)
	}
	id := ulid.Make().String()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return nil, fmt.Errorf("begin transaction: %w", err) }
	defer tx.Rollback()
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(id, username, email, password_hash, role, status, quota_mode, quota_bytes, created_at, updated_at) VALUES(?, ?, ?, '', 'REGULAR', 'PENDING_ACTIVATION', ?, ?, ?, ?)`, id, username, email, mode, quotaBytes, now, now); err != nil { return nil, fmt.Errorf("insert user: %w", err) }
 	secret, tokenHash, tokenID, err := database.GenerateAuthToken()
 	if err != nil { return nil, fmt.Errorf("generate invitation token: %w", err) }
 	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_tokens(id, user_id, token_hash, token_type, expires_at) VALUES(?, ?, ?, 'INVITATION', ?)`, tokenID, id, tokenHash, now.Add(24*time.Hour)); err != nil { return nil, fmt.Errorf("insert invitation token: %w", err) }
	if err := tx.Commit(); err != nil { return nil, fmt.Errorf("commit user creation: %w", err) }
	var quotaGB *float64
	if quotaBytes >= 0 { q := float64(quotaBytes) / (1024.0*1024.0*1024.0); quotaGB = &q }
	return &AdminUserResponse{ID:id, Username:username, Email:email, Role:"REGULAR", Status:"PENDING_ACTIVATION", QuotaMode:mode, QuotaBytes:quotaBytes, QuotaGB:quotaGB, CreatedAt:now.Format(time.RFC3339), InvitationToken:secret, InvitationURL:"/set-password#token="+secret}, nil
}

// UpdateUserQuota updates quota mode and quota bytes for a regular user
func (s *Service) UpdateUserQuota(ctx context.Context, userID string, req *UpdateUserQuotaRequest) (*AdminUserResponse, error) {
	var user struct {
		ID         string
		Username   string
		Email      string
		Role       string
		QuotaMode  string
		QuotaBytes int64
		CreatedAt  time.Time
	}
	err := s.db.QueryRowContext(ctx, `SELECT id, username, email, role, COALESCE(quota_mode, 'DEFAULT'), quota_bytes, created_at FROM users WHERE id = ?`, userID).
		Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.QuotaMode, &user.QuotaBytes, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if user.Role == "SUPER_ADMIN" {
		return nil, fmt.Errorf("super admin quota is locked to unlimited")
	}

	mode := strings.ToUpper(strings.TrimSpace(req.QuotaMode))
	var quotaBytes int64
	switch mode {
	case "DEFAULT":
		defaultBytes, err := s.GetDefaultQuotaBytes(ctx)
		if err != nil {
			return nil, fmt.Errorf("get default quota: %w", err)
		}
		quotaBytes = defaultBytes
	case "CUSTOM":
		if req.QuotaGB == nil || *req.QuotaGB < 0 {
			return nil, fmt.Errorf("quota_gb must be greater than or equal to 0")
		}
		quotaBytes = int64(math.Round(*req.QuotaGB * 1024 * 1024 * 1024))
	case "UNLIMITED":
		quotaBytes = -1
	default:
		return nil, fmt.Errorf("invalid quota mode: %s", req.QuotaMode)
	}

	_, err = s.db.ExecContext(ctx, `UPDATE users SET quota_mode = ?, quota_bytes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, mode, quotaBytes, userID)
	if err != nil {
		return nil, fmt.Errorf("update user quota: %w", err)
	}

	var usedBytes int64
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM (
			SELECT disk_key, MAX(size_bytes) AS size_bytes
			FROM my_storage_items
			WHERE owner_id = ? AND kind = 'file' AND disk_key IS NOT NULL
			GROUP BY disk_key
		)
	`, userID).Scan(&usedBytes)
	usedGB := float64(usedBytes) / (1024.0 * 1024.0 * 1024.0)

	var quotaGB *float64
	if quotaBytes >= 0 {
		q := float64(quotaBytes) / (1024.0 * 1024.0 * 1024.0)
		quotaGB = &q
	}

	return &AdminUserResponse{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		Role:       user.Role,
		QuotaMode:  mode,
		QuotaBytes: quotaBytes,
		QuotaGB:    quotaGB,
		UsedBytes:  usedBytes,
		UsedGB:     usedGB,
		CreatedAt:  user.CreatedAt.Format(time.RFC3339),
	}, nil
}

// DeleteUser tombstones a user, invalidates access, and permanently purges their personal files
func (s *Service) DeleteUser(ctx context.Context, actorID, targetID string, req *DeleteUserRequest) error {
	if actorID != "" && actorID == targetID {
		return fmt.Errorf("cannot delete your own account")
	}

	var role, email string
	err := s.db.QueryRowContext(ctx, `SELECT role, email FROM users WHERE id = ? AND deleted_at IS NULL`, targetID).Scan(&role, &email)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("user not found")
	}
	if err != nil {
		return fmt.Errorf("query user: %w", err)
	}

	if role == "SUPER_ADMIN" {
		return fmt.Errorf("super admin cannot be deleted")
	}

	if req == nil || req.Email != email {
		return fmt.Errorf("confirmation email does not match")
	}

	type blobCandidate struct {
		diskKey  string
		nodeID   string
		nodePath string
	}
	var candidates []blobCandidate
	bRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT i.disk_key, COALESCE(i.storage_node_id, '') AS node_id, COALESCE(sn.path, '') AS node_path
		FROM my_storage_items i
		LEFT JOIN storage_nodes sn ON sn.id = i.storage_node_id
		WHERE i.owner_id = ? AND i.kind = 'file' AND i.disk_key IS NOT NULL AND i.disk_key != ''
	`, targetID)
	if err == nil {
		for bRows.Next() {
			var bc blobCandidate
			if err := bRows.Scan(&bc.diskKey, &bc.nodeID, &bc.nodePath); err == nil && bc.diskKey != "" {
				candidates = append(candidates, bc)
			}
		}
		bRows.Close()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Clean up stars on user's items or created by user
	_, _ = tx.ExecContext(ctx, `DELETE FROM vfs_starred WHERE user_id = ? OR node_id IN (SELECT id FROM my_storage_items WHERE owner_id = ?)`, targetID, targetID)

	// Permanently delete all personal files & folders (including trash)
	if _, err := tx.ExecContext(ctx, `DELETE FROM my_storage_items WHERE owner_id = ?`, targetID); err != nil {
		return fmt.Errorf("delete user items: %w", err)
	}

	// Tombstone the user: scramble credentials, mark deleted_at, preserve ID for organization_items foreign keys
	tombstoneEmail := fmt.Sprintf("deleted_%s@cubycloud.local", targetID)
	tombstoneUsername := fmt.Sprintf("deleted_user_%s", targetID)
	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET deleted_at = CURRENT_TIMESTAMP,
		    username = ?,
		    email = ?,
		    password_hash = '',
		    recovery_key_hash = NULL,
		    quota_bytes = 0,
		    quota_mode = 'DEFAULT',
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, tombstoneUsername, tombstoneEmail, targetID)
	if err != nil {
		return fmt.Errorf("tombstone user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Physical blob cleanup: delete only if unreferenced by any other my_storage_items or organization_items
	if s.storage != nil {
		deletedKeys := make(map[string]bool)
		for _, bc := range candidates {
			if deletedKeys[bc.diskKey] {
				continue
			}
			var refCount int
			err := s.db.QueryRowContext(ctx, `
				SELECT (SELECT COUNT(*) FROM my_storage_items WHERE disk_key = ?) +
				       (SELECT COUNT(*) FROM organization_items WHERE disk_key = ?)
			`, bc.diskKey, bc.diskKey).Scan(&refCount)
			if err == nil && refCount == 0 {
				deletedKeys[bc.diskKey] = true
				_ = storage.DeleteBlob(ctx, s.db, s.storage, bc.nodeID, bc.nodePath, bc.diskKey)
			}
		}
	}

	return nil
}

// ResetUserPassword invalidates existing sessions and issues a one-time reset token.
func (s *Service) ResetUserPassword(ctx context.Context, actorID, targetID string) (string, time.Time, error) {
	if strings.TrimSpace(targetID) == "" { return "", time.Time{}, fmt.Errorf("user ID required") }
	if actorID != "" && actorID == targetID { return "", time.Time{}, fmt.Errorf("cannot reset your own password") }
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ? AND deleted_at IS NULL`, targetID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) { return "", time.Time{}, fmt.Errorf("user not found") }
	if err != nil { return "", time.Time{}, fmt.Errorf("query user: %w", err) }
 	secret, tokenHash, tokenID, err := database.GenerateAuthToken()
 	if err != nil { return "", time.Time{}, fmt.Errorf("generate reset token: %w", err) }
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return "", time.Time{}, fmt.Errorf("begin transaction: %w", err) }
	defer tx.Rollback()
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE auth_tokens SET consumed_at = ? WHERE user_id = ? AND token_type = 'PASSWORD_RESET' AND consumed_at IS NULL`, now, targetID); err != nil { return "", time.Time{}, fmt.Errorf("revoke previous reset tokens: %w", err) }
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = '', token_version = token_version + 1, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, now, targetID); err != nil { return "", time.Time{}, fmt.Errorf("disable user password: %w", err) }
	expires := now.Add(30*time.Minute)
 	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_tokens(id, user_id, token_hash, token_type, expires_at) VALUES(?, ?, ?, 'PASSWORD_RESET', ?)`, tokenID, targetID, tokenHash, expires); err != nil { return "", time.Time{}, fmt.Errorf("insert reset token: %w", err) }
	if err := tx.Commit(); err != nil { return "", time.Time{}, fmt.Errorf("commit reset token: %w", err) }
	return secret, expires, nil
}
// RegenerateInvitation issues a fresh invitation for a pending user.
func (s *Service) RegenerateInvitation(ctx context.Context, targetID string) (string, time.Time, error) {
	if strings.TrimSpace(targetID) == "" {
		return "", time.Time{}, fmt.Errorf("user ID required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ? AND deleted_at IS NULL`, targetID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, fmt.Errorf("user not found")
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("query user: %w", err)
	}
	if status != "PENDING_ACTIVATION" {
		return "", time.Time{}, fmt.Errorf("user is not pending activation")
	}

	secret, tokenHash, tokenID, err := database.GenerateAuthToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate invitation token: %w", err)
	}
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE auth_tokens SET consumed_at = ? WHERE user_id = ? AND token_type = 'INVITATION' AND consumed_at IS NULL`, now, targetID); err != nil {
		return "", time.Time{}, fmt.Errorf("consume previous invitation tokens: %w", err)
	}
	expires := now.Add(24 * time.Hour)
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_tokens(id, user_id, token_hash, token_type, expires_at) VALUES(?, ?, ?, 'INVITATION', ?)`, tokenID, targetID, tokenHash, expires); err != nil {
		return "", time.Time{}, fmt.Errorf("insert invitation token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", time.Time{}, fmt.Errorf("commit invitation token: %w", err)
	}
	return secret, expires, nil
}

func detectFileCategory(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	switch ext {
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "3gp", "ts", "mpg", "mpeg":
		return "video"
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "heic", "heif", "tiff", "tif", "ico", "avif":
		return "image"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv", "rtf", "odt", "ods", "odp", "md", "html", "htm", "xml", "json", "yaml", "yml":
		return "document"
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "tgz", "iso", "dmg":
		return "archive"
	default:
		return "other"
	}
}

// GetAdminStorageStats returns system-wide storage stats: total allocated capacity across all nodes,
// total usage across all users' files, category breakdown (including isolated trash), free space, and node count.
func (s *Service) GetAdminStorageStats(ctx context.Context) (*AdminStorageStatsResponse, error) {
	var totalCapacityBytes int64
	var nodesCount int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(capacity_bytes), 0), COUNT(*) FROM storage_nodes`).Scan(&totalCapacityBytes, &nodesCount)
	if err != nil {
		return nil, fmt.Errorf("query storage nodes stats: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			disk_key AS k,
			MAX(size_bytes) AS size_bytes,
			MAX(CASE WHEN deleted_at IS NULL THEN name ELSE '' END) AS active_name,
			MAX(name) AS any_name,
			COUNT(CASE WHEN deleted_at IS NULL THEN 1 END) AS active_count
		FROM (
			SELECT disk_key, size_bytes, name, deleted_at FROM my_storage_items
			WHERE kind = 'file' AND disk_key IS NOT NULL
			UNION ALL
			SELECT disk_key, size_bytes, name, deleted_at FROM organization_items
			WHERE kind = 'file' AND disk_key IS NOT NULL
		) AS items
		GROUP BY disk_key
	`)
	if err != nil {
		return nil, fmt.Errorf("query storage usage stats: %w", err)
	}
	defer rows.Close()

	var (
		videoBytes     int64
		imageBytes     int64
		documentBytes  int64
		archiveBytes   int64
		otherBytes     int64
		trashBytes     int64
		totalUsedBytes int64
	)

	for rows.Next() {
		var (
			k           string
			sizeBytes   int64
			activeName  string
			anyName     string
			activeCount int
		)
		if err := rows.Scan(&k, &sizeBytes, &activeName, &anyName, &activeCount); err != nil {
			return nil, fmt.Errorf("scan category usage: %w", err)
		}
		totalUsedBytes += sizeBytes

		if activeCount == 0 {
			trashBytes += sizeBytes
			continue
		}

		name := activeName
		if name == "" {
			name = anyName
		}
		cat := detectFileCategory(name)
		switch cat {
		case "video":
			videoBytes += sizeBytes
		case "image":
			imageBytes += sizeBytes
		case "document":
			documentBytes += sizeBytes
		case "archive":
			archiveBytes += sizeBytes
		default:
			otherBytes += sizeBytes
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows category usage: %w", err)
	}

	const bytesInGB = 1024.0 * 1024.0 * 1024.0
	toGB := func(b int64) float64 {
		return float64(b) / bytesInGB
	}

	categories := []AdminStorageCategoryUsage{
		{ID: "video", Label: "Video", Bytes: videoBytes, UsedGB: toGB(videoBytes)},
		{ID: "image", Label: "Gambar", Bytes: imageBytes, UsedGB: toGB(imageBytes)},
		{ID: "document", Label: "Dokumen", Bytes: documentBytes, UsedGB: toGB(documentBytes)},
		{ID: "archive", Label: "Arsip", Bytes: archiveBytes, UsedGB: toGB(archiveBytes)},
	}
	if otherBytes > 0 {
		categories = append(categories, AdminStorageCategoryUsage{
			ID:     "other",
			Label:  "Lainnya",
			Bytes:  otherBytes,
			UsedGB: toGB(otherBytes),
		})
	}
	categories = append(categories, AdminStorageCategoryUsage{
		ID:     "trash",
		Label:  "Trash",
		Bytes:  trashBytes,
		UsedGB: toGB(trashBytes),
	})

	totalCapacityGB := float64(totalCapacityBytes) / bytesInGB
	totalUsedGB := float64(totalUsedBytes) / bytesInGB
	freeBytes := totalCapacityBytes - totalUsedBytes
	if freeBytes < 0 {
		freeBytes = 0
	}
	freeGB := float64(freeBytes) / bytesInGB

	return &AdminStorageStatsResponse{
		TotalCapacityBytes: totalCapacityBytes,
		TotalCapacityGB:    totalCapacityGB,
		TotalUsedBytes:     totalUsedBytes,
		TotalUsedGB:        totalUsedGB,
		FreeBytes:          freeBytes,
		FreeGB:             freeGB,
		NodesCount:         nodesCount,
		Categories:         categories,
	}, nil
}

// GetAdminTopStorageUsers returns paginated non-deleted users sorted by storage usage descending.
func (s *Service) GetAdminTopStorageUsers(ctx context.Context, limit, offset int) (*AdminTopStorageUsersResponse, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	if total == 0 || int64(offset) >= total {
		return &AdminTopStorageUsersResponse{
			Items:      make([]AdminTopStorageUserItem, 0),
			Total:      total,
			HasMore:    false,
			NextOffset: offset,
		}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			u.id,
			u.username,
			u.email,
			u.role,
			COALESCE(u.quota_mode, 'DEFAULT') AS quota_mode,
			u.quota_bytes,
			u.created_at,
			COALESCE(usage.used_bytes, 0) AS used_bytes
		FROM users u
		LEFT JOIN (
			SELECT owner_id, COALESCE(SUM(size_bytes), 0) AS used_bytes
			FROM (
				SELECT owner_id, disk_key, MAX(size_bytes) AS size_bytes
				FROM my_storage_items
				WHERE kind = 'file' AND disk_key IS NOT NULL
				GROUP BY owner_id, disk_key
			)
			GROUP BY owner_id
		) usage ON usage.owner_id = u.id
		WHERE u.deleted_at IS NULL
		ORDER BY used_bytes DESC, u.created_at ASC, u.id ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query top storage users: %w", err)
	}
	defer rows.Close()

	items := make([]AdminTopStorageUserItem, 0, limit)
	const bytesInGB = 1024.0 * 1024.0 * 1024.0

	for rows.Next() {
		var (
			id           string
			username     string
			email        string
			role         string
			quotaMode    string
			quotaBytes   int64
			rawCreatedAt any
			usedBytes    int64
		)
		if err := rows.Scan(&id, &username, &email, &role, &quotaMode, &quotaBytes, &rawCreatedAt, &usedBytes); err != nil {
			return nil, fmt.Errorf("scan top storage user: %w", err)
		}

		if role == "SUPER_ADMIN" {
			quotaMode = "UNLIMITED"
			quotaBytes = -1
		}

		var quotaGB *float64
		if quotaBytes >= 0 {
			q := float64(quotaBytes) / bytesInGB
			quotaGB = &q
		}

		usedGB := float64(usedBytes) / bytesInGB

		items = append(items, AdminTopStorageUserItem{
			ID:         id,
			Username:   username,
			Email:      email,
			Role:       role,
			QuotaMode:  quotaMode,
			QuotaBytes: quotaBytes,
			QuotaGB:    quotaGB,
			UsedBytes:  usedBytes,
			UsedGB:     usedGB,
			CreatedAt:  formatCreatedAt(rawCreatedAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	nextOffset := offset + len(items)
	hasMore := int64(nextOffset) < total

	return &AdminTopStorageUsersResponse{
		Items:      items,
		Total:      total,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}, nil
}

func formatCreatedAt(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format(time.RFC3339)
	case string:
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed.Format(time.RFC3339)
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
			return parsed.Format(time.RFC3339)
		}
		return t
	case []byte:
		s := string(t)
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.Format(time.RFC3339)
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
			return parsed.Format(time.RFC3339)
		}
		return s
	default:
		return ""
	}
}
