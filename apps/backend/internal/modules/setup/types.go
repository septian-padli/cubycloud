package setup

type AdminUser struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	RecoveryKey string `json:"recovery_key"` // Client-generated recovery key
}

type StorageNodeInput struct {
	Path          string `json:"path"`
	CapacityBytes int64  `json:"capacity_bytes"`
	Enabled       bool   `json:"enabled"`
}

type CloudConfig struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	NodeName  string `json:"node_name"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	CapacityCapGB int64  `json:"capacity_cap_gb,omitempty"`
}

type CompleteSetupRequest struct {
	Admin             AdminUser          `json:"admin"`
	StorageNodes      []StorageNodeInput `json:"storage_nodes"`
	PlacementStrategy string             `json:"placement_strategy"`
	Cloud             CloudConfig        `json:"cloud"`
}

type CompleteSetupResponse struct {
	UserID      string `json:"-"` // Internal use only for JWT generation
	RecoveryKey string `json:"recovery_key"`
	Redirect    string `json:"redirect"`
}

type StorageNodeScan struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	CapacityBytes int64  `json:"capacity_bytes"`
	FreeBytes     int64  `json:"free_bytes"`
}

type ScanResponse struct {
	Nodes   []StorageNodeScan `json:"nodes"`
	Devices []interface{}     `json:"devices"` // ponytail: empty for v1.0
}

type StatusResponse struct {
	IsSetupCompleted bool `json:"is_setup_completed"`
}
