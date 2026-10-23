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
	PlacementStrategy  *string      `json:"placement_strategy,omitempty"`
}
type StorageNodeResponse struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Kind          string `json:"kind"` // "SSD", "HDD", "S3"
	Endpoint      string `json:"endpoint,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
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

type AdminUserResponse struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	QuotaMode  string    `json:"quota_mode"`
	QuotaBytes int64     `json:"quota_bytes"`
	QuotaGB    *float64  `json:"quota_gb,omitempty"`
	UsedBytes  int64     `json:"used_bytes"`
	UsedGB     float64   `json:"used_gb"`
	CreatedAt  string    `json:"created_at"`
}

type AdminUsersListResponse struct {
	Users []AdminUserResponse `json:"users"`
}

type CreateUserRequest struct {
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	Password  string   `json:"password,omitempty"`
	QuotaMode string   `json:"quota_mode"`
	QuotaGB   *float64 `json:"quota_gb,omitempty"`
}

type UpdateUserQuotaRequest struct {
	QuotaMode string   `json:"quota_mode"`
	QuotaGB   *float64 `json:"quota_gb,omitempty"`
}

type DeleteUserRequest struct {
	Email string `json:"email"`
}

// AdminStorageCategoryUsage represents storage usage for a specific file category
type AdminStorageCategoryUsage struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Bytes  int64   `json:"bytes"`
	UsedGB float64 `json:"used_gb"`
}

// AdminStorageStatsResponse contains system-wide storage stats for super admin
type AdminStorageStatsResponse struct {
	TotalCapacityBytes int64                       `json:"total_capacity_bytes"`
	TotalCapacityGB    float64                     `json:"total_capacity_gb"`
	TotalUsedBytes     int64                       `json:"total_used_bytes"`
	TotalUsedGB        float64                     `json:"total_used_gb"`
	FreeBytes          int64                       `json:"free_bytes"`
	FreeGB             float64                     `json:"free_gb"`
	NodesCount         int                         `json:"nodes_count"`
	Categories         []AdminStorageCategoryUsage `json:"categories"`
}
// AdminTopStorageUserItem represents a user with storage usage in the top storage list
type AdminTopStorageUserItem struct {
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	Email      string   `json:"email"`
	Role       string   `json:"role"`
	QuotaMode  string   `json:"quota_mode"`
	QuotaBytes int64    `json:"quota_bytes"`
	QuotaGB    *float64 `json:"quota_gb,omitempty"`
	UsedBytes  int64    `json:"used_bytes"`
	UsedGB     float64  `json:"used_gb"`
	CreatedAt  string   `json:"created_at"`
}

// AdminTopStorageUsersResponse contains paginated top storage users
type AdminTopStorageUsersResponse struct {
	Items      []AdminTopStorageUserItem `json:"items"`
	Total      int64                     `json:"total"`
	HasMore    bool                      `json:"has_more"`
	NextOffset int                       `json:"next_offset"`
}
