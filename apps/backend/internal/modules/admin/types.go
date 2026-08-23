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
	Enabled       bool   `json:"enabled"`
}

type StorageNodesListResponse struct {
	Nodes []StorageNodeResponse `json:"nodes"`
}

type DisableNodeRequest struct {
	NodeID string `json:"node_id"`
}
