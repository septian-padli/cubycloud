package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

const (
	SettingDefaultQuotaGB      = "default_user_quota_gb"
	SettingAuditRetentionDays  = "audit_log_retention_days"
	SettingWorkerConcurrency   = "worker_concurrency_limit"
	SettingPlacementStrategy   = "placement_strategy"
)

type Service struct {
	repo        *Repository
	db          *bun.DB
	storage     storage.StorageDriver
	dataDirPath string
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
		return nil, fmt.Errorf("get default quota: %w", err)
	}
	
	retentionDays, err := s.repo.GetSetting(ctx, SettingAuditRetentionDays)
	if err != nil {
		return nil, fmt.Errorf("get audit retention: %w", err)
	}
	
	concurrency, err := s.repo.GetSetting(ctx, SettingWorkerConcurrency)
	if err != nil {
		return nil, fmt.Errorf("get worker concurrency: %w", err)
	}
	
	strategy, err := s.repo.GetSetting(ctx, SettingPlacementStrategy)
	if err != nil {
		return nil, fmt.Errorf("get placement strategy: %w", err)
	}

	quotaValue, _ := strconv.ParseInt(quotaGB, 10, 64)
	retentionValue, _ := strconv.Atoi(retentionDays)
	concurrencyValue, _ := strconv.Atoi(concurrency)

	quota := QuotaPolicy{
		Mode: "limited",
		GB:   int(quotaValue),
	}
	if quotaValue == -1 {
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

// GetStorageNodes lists all storage nodes with capacity info
func (s *Service) GetStorageNodes(ctx context.Context) ([]StorageNodeResponse, error) {
	nodes, err := s.repo.GetAllStorageNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get storage nodes: %w", err)
	}

	var response []StorageNodeResponse
	for _, node := range nodes {
		nodeResp := StorageNodeResponse{
			ID:            node.ID,
			Type:          node.Type,
			CapacityCapGB: node.CapacityBytes / (1024 * 1024 * 1024),
			Enabled:       node.Enabled,
		}

		if node.Type == "LOCAL" && node.Path != nil {
			nodeResp.Path = *node.Path
			nodeResp.Name = filepath.Base(*node.Path)
			nodeResp.Kind = detectDiskKind(*node.Path)

			// Get current free space
			capacity, freeSpace, err := s.storage.StatLocal(ctx, *node.Path)
			if err == nil {
				nodeResp.TotalGB = capacity / (1024 * 1024 * 1024)
				nodeResp.FreeGB = freeSpace / (1024 * 1024 * 1024)
			}
		} else if node.Type == "S3" {
			if node.Endpoint != nil {
				nodeResp.Name = fmt.Sprintf("S3: %s", *node.Bucket)
			} else {
				nodeResp.Name = fmt.Sprintf("S3: %s", *node.Bucket)
			}
			nodeResp.Path = ""
			nodeResp.Kind = "S3"
			nodeResp.TotalGB = node.CapacityBytes / (1024 * 1024 * 1024)
			nodeResp.FreeGB = nodeResp.TotalGB // ponytail: S3 free space = capacity cap
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

			node := &database.StorageNode{
				ID:            ulid.Make().String(),
				Type:          "LOCAL",
				Path:          &defaultPath,
				CapacityBytes: capacity,
				Enabled:       true,
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
			node := &database.StorageNode{
				ID:            ulid.Make().String(),
				Type:          "LOCAL",
				Path:          &nodePath,
				CapacityBytes: capacity,
				Enabled:       true,
			}
			if err := s.repo.UpsertStorageNode(ctx, node); err != nil {
				continue
			}
		}
	}

	return s.GetStorageNodes(ctx)
}

// detectDiskKind returns "SSD" or "HDD" based on path heuristic
func detectDiskKind(path string) string {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "ssd") || strings.Contains(lower, "nvme") {
		return "SSD"
	}
	return "HDD"
}


// CreateLocalNode adds a new local disk storage node
func (s *Service) CreateLocalNode(ctx context.Context, req *CreateLocalNodeRequest) (*StorageNodeResponse, error) {
	// Validate path exists
	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", req.Path)
	}

	// Check duplicate path
	existing, err := s.repo.GetNodeByPath(ctx, req.Path)
	if err != nil {
		return nil, fmt.Errorf("check existing node: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("node with path %s already exists", req.Path)
	}

	// Create node
	name := filepath.Base(req.Path)
	node := &database.StorageNode{
		ID:            ulid.Make().String(),
		Type:          "LOCAL",
		Path:          &req.Path,
		CapacityBytes: req.CapacityCapGB * 1024 * 1024 * 1024,
		Enabled:       req.Enabled,
	}

	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	// Get disk stats using StatLocal (returns capacity and free space)
	kind := detectDiskKind(req.Path)
	capacity, free, err := s.storage.StatLocal(ctx, req.Path)
	totalGB := capacity / (1024 * 1024 * 1024)
	freeGB := free / (1024 * 1024 * 1024)
	
	// Log stat error but don't fail (node creation still succeeds)
	if err != nil {
		totalGB = 0
		freeGB = 0
	}

	return &StorageNodeResponse{
		ID:            node.ID,
		Type:          "LOCAL",
		Name:          name,
		Path:          req.Path,
		Kind:          kind,
		TotalGB:       totalGB,
		FreeGB:        freeGB,
		CapacityCapGB: req.CapacityCapGB,
		Enabled:       req.Enabled,
	}, nil
}

// CreateCloudNode adds a new S3-compatible cloud storage node
func (s *Service) CreateCloudNode(ctx context.Context, req *CreateCloudNodeRequest) (*StorageNodeResponse, error) {
	// Check duplicate bucket
	existing, err := s.repo.GetNodeByName(ctx, req.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check existing node: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("node with bucket %s already exists", req.Bucket)
	}

	// Simple encryption placeholder (use proper encryption in production)
	accessKeyEnc := []byte(req.AccessKey)
	secretKeyEnc := []byte(req.SecretKey)

	node := &database.StorageNode{
		ID:                 ulid.Make().String(),
		Type:               "S3",
		Endpoint:           &req.Endpoint,
		Bucket:             &req.Bucket,
		Region:             &req.Region,
		AccessKeyEncrypted: accessKeyEnc,
		SecretKeyEncrypted: secretKeyEnc,
		CapacityBytes:      req.CapacityCapGB * 1024 * 1024 * 1024,
		Enabled:            req.Enabled,
	}

	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	return &StorageNodeResponse{
		ID:            node.ID,
		Type:          "S3",
		Name:          req.Name,
		Path:          "",
		Kind:          "S3",
		TotalGB:       req.CapacityCapGB,
		FreeGB:        req.CapacityCapGB,
		CapacityCapGB: req.CapacityCapGB,
		Enabled:       req.Enabled,
	}, nil
}

// UpdateNode updates storage node capacity and/or enabled status
func (s *Service) UpdateNode(ctx context.Context, nodeID string, req *UpdateNodeRequest) error {
	node, err := s.repo.GetStorageNodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	// Update fields if provided
	if req.CapacityCapGB != nil {
		// Validate capacity >= 1GB
		if *req.CapacityCapGB < 1 {
			return fmt.Errorf("capacity must be at least 1 GB")
		}
		node.CapacityBytes = *req.CapacityCapGB * 1024 * 1024 * 1024
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
