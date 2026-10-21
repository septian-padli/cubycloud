package database

import (
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID              string    `bun:"id,pk"`
	Username        string    `bun:"username,notnull,unique"`
	Email           string    `bun:"email,notnull,unique"`
	DisplayName     string    `bun:"display_name,notnull,default:''"`
	PasswordHash    string    `bun:"password_hash,notnull"`
	RecoveryKeyHash *string   `bun:"recovery_key_hash"`
	Role            string    `bun:"role,notnull"`
	TokenVersion    int       `bun:"token_version,notnull,default:0"`
	QuotaMode       string    `bun:"quota_mode,notnull,default:'DEFAULT'"`
	QuotaBytes      int64     `bun:"quota_bytes,notnull,default:-1"`
	CreatedAt       time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,notnull,default:current_timestamp"`
	DeletedAt       *time.Time `bun:"deleted_at"`
}

type StorageNode struct {
	bun.BaseModel `bun:"table:storage_nodes,alias:sn"`

	ID                  string    `bun:"id,pk"`
	Name                string    `bun:"name,notnull,default:''"`
	Type                string    `bun:"type,notnull"`
	Path                *string   `bun:"path"`
	Endpoint            *string   `bun:"endpoint"`
	Bucket              *string   `bun:"bucket"`
	Region              *string   `bun:"region"`
	AccessKeyEncrypted  []byte    `bun:"access_key_encrypted,type:blob"`
	SecretKeyEncrypted  []byte    `bun:"secret_key_encrypted,type:blob"`
	CapacityBytes       int64     `bun:"capacity_bytes,notnull"`
	Enabled             bool      `bun:"enabled,notnull,default:1"`
	CreatedAt           time.Time `bun:"created_at,notnull,default:current_timestamp"`
}

type Setting struct {
	bun.BaseModel `bun:"table:settings,alias:s"`

	Key   string `bun:"key,pk"`
	Value string `bun:"value,notnull"`
}

type MyStorageItem struct {
	bun.BaseModel `bun:"table:my_storage_items,alias:msi"`

	ID            string     `bun:"id,pk"`
	OwnerID       string     `bun:"owner_id,notnull"`
	Name          string     `bun:"name,notnull"`
	Kind          string     `bun:"kind,notnull"`
	SizeBytes     int64      `bun:"size_bytes,notnull,default:0"`
	ParentID      *string    `bun:"parent_id"`
	DiskKey       *string    `bun:"disk_key"`
	StorageNodeID *string    `bun:"storage_node_id"`
	ModifiedAt    time.Time  `bun:"modified_at,notnull"`
	DeletedAt     *time.Time `bun:"deleted_at"`
}

type OrganizationItem struct {
	bun.BaseModel `bun:"table:organization_items,alias:oi"`

	ID            string     `bun:"id,pk"`
	CreatedBy     string     `bun:"created_by,notnull"`
	Name          string     `bun:"name,notnull"`
	Kind          string     `bun:"kind,notnull"`
	SizeBytes     int64      `bun:"size_bytes,notnull,default:0"`
	ParentID      *string    `bun:"parent_id"`
	DiskKey       *string    `bun:"disk_key"`
	StorageNodeID *string    `bun:"storage_node_id"`
	ModifiedAt    time.Time  `bun:"modified_at,notnull"`
	DeletedAt     *time.Time `bun:"deleted_at"`
}

type ShareGrant struct {
	bun.BaseModel `bun:"table:share_grants,alias:sg"`

	ID         string    `bun:"id,pk"`
	ItemID     string    `bun:"item_id,notnull"`
	UserID     string    `bun:"user_id,notnull"`
	Permission string    `bun:"permission,notnull"`
	CreatedAt  time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt  time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

type ShareLink struct {
	bun.BaseModel `bun:"table:share_links,alias:sl"`

	ID           string     `bun:"id,pk"`
	ItemID       string     `bun:"item_id,notnull"`
	Token        string     `bun:"token,notnull,unique"`
	Permission   string     `bun:"permission,notnull"`
	PasswordHash *string    `bun:"password_hash"`
	ExpiresAt    *time.Time `bun:"expires_at"`
	CreatedBy    string     `bun:"created_by,notnull"`
	CreatedAt    time.Time  `bun:"created_at,notnull,default:current_timestamp"`
}

type ActivityLog struct {
	bun.BaseModel `bun:"table:activity_logs,alias:al"`

	ID        string    `bun:"id,pk"`
	UserID    string    `bun:"user_id,notnull"`
	Action    string    `bun:"action,notnull"`
	ItemID    string    `bun:"item_id,notnull"`
	ItemName  string    `bun:"item_name,notnull"`
	ItemKind  string    `bun:"item_kind,notnull"`
	Source    string    `bun:"source,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
}

type RecentAccess struct {
	bun.BaseModel `bun:"table:vfs_recent_access,alias:vra"`

	ID         string    `bun:"id,pk"`
	UserID     string    `bun:"user_id,notnull"`
	ItemID     string    `bun:"item_id,notnull"`
	Source     string    `bun:"source,notnull"`
	AccessedAt time.Time `bun:"accessed_at,notnull,default:current_timestamp"`
}
