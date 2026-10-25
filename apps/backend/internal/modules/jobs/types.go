package jobs

import "time"

const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"

	TypeCompressZip   = "compress_zip"
	TypeNodeMigration = "node_migration"
)

type NodeMigrationPayload struct {
	SourceNodeID string `json:"source_node_id"`
	TargetNodeID string `json:"target_node_id,omitempty"`
}

type NodeMigrationResult struct {
	SourceNodeID     string `json:"source_node_id"`
	TargetNodeID     string `json:"target_node_id,omitempty"`
	TotalFiles       int    `json:"total_files"`
	MigratedFiles    int    `json:"migrated_files"`
	FailedFiles      int    `json:"failed_files"`
	SafeToDisconnect bool   `json:"safe_to_disconnect"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

type CompressZipPayload struct {
	Scope          string   `json:"scope"`            // "my_storage" or "organization"
	UserID         string   `json:"user_id"`
	ItemIDs        []string `json:"item_ids"`
	TargetParentID *string  `json:"target_parent_id"` // Action origin folder
	TargetName     string   `json:"target_name"`      // Default e.g. "Archive.zip"
}

type CompressZipResult struct {
	ItemID    string  `json:"item_id"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parent_id"`
	SizeBytes int64   `json:"size_bytes"`
	Scope     string  `json:"scope"`
}

type JobResponse struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	UserID       string     `json:"user_id"`
	Progress     int        `json:"progress"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	Result       any        `json:"result,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}
