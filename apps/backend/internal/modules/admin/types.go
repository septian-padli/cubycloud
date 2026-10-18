package admin

type QuotaPolicy struct {
	Mode string `json:"mode"` // "limited" or "unlimited"
	GB   int    `json:"gb"`
}

type SystemSettingsResponse struct {
	DefaultQuota       QuotaPolicy `json:"default_quota"`
	AuditRetentionDays int         `json:"audit_retention_days"`
	WorkerConcurrency  int         `json:"worker_concurrency"`
	PlacementStrategy  string      `json:"placement_strategy"`
}

type UpdateSettingsRequest struct {
	DefaultQuota       *QuotaPolicy `json:"default_quota,omitempty"`
	AuditRetentionDays *int         `json:"audit_retention_days,omitempty"`
	WorkerConcurrency  *int         `json:"worker_concurrency,omitempty"`
}

type StorageNodeResponse struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Kind          string `json:"kind"` // "SSD", "HDD", "S3"
	TotalGB       int64  `json:"total_gb"`
	FreeGB        int64  `json:"free_gb"`
	CapacityCapGB int64  `json:"capacity_cap_gb"`
	UsedBytes     int64  `json:"used_bytes"`
	Enabled       bool   `json:"enabled"`
}

type StorageNodesListResponse struct {
	Nodes []StorageNodeResponse `json:"nodes"`
}

type DisableNodeRequest struct {
	NodeID string `json:"node_id"`
}

// CreateLocalNodeRequest - Add local disk storage node
type CreateLocalNodeRequest struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	CapacityCapGB int64  `json:"capacity_cap_gb"`
	Enabled       bool   `json:"enabled"`
}

// CreateCloudNodeRequest - Add S3-compatible cloud storage node
type CreateCloudNodeRequest struct {
	Name          string `json:"name"`
	Endpoint      string `json:"endpoint"`
	Bucket        string `json:"bucket"`
	Region        string `json:"region"`
	AccessKey     string `json:"access_key"`
	SecretKey     string `json:"secret_key"`
	CapacityCapGB int64  `json:"capacity_cap_gb"`
	Enabled       bool   `json:"enabled"`
}

// UpdateNodeRequest - Update storage node
type UpdateNodeRequest struct {
	Name          *string `json:"name,omitempty"`
	CapacityCapGB *int64  `json:"capacity_cap_gb,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}
