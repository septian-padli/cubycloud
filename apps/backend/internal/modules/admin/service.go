package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/cubycloud/backend/internal/database"
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

type Service struct {
	repo        *Repository
	db          *bun.DB
	storage     storage.StorageDriver
	dataDirPath string
	nodeNameMu  sync.Mutex
}

func NewService(db *bun.DB, storage storage.StorageDriver, dataDirPath string) *Service {
	return &Service{
		repo:        NewRepository(db),
		db:          db,
		storage:     storage,
		dataDirPath: dataDirPath,
	}
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
			// Update capacity
			existingNode.CapacityBytes = capacity
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
	node := &database.StorageNode{
		ID: ulid.Make().String(), Name: name, Type: "S3", Endpoint: &req.Endpoint,
		Bucket: &req.Bucket, Region: &req.Region, AccessKeyEncrypted: []byte(req.AccessKey),
		SecretKeyEncrypted: []byte(req.SecretKey),
		CapacityBytes:      req.CapacityCapGB * 1024 * 1024 * 1024, Enabled: req.Enabled,
	}
	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return &StorageNodeResponse{
		ID: node.ID, Type: "S3", Name: name, Kind: "S3", TotalGB: req.CapacityCapGB,
		FreeGB: req.CapacityCapGB, CapacityCapGB: req.CapacityCapGB, Enabled: req.Enabled,
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
