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
	PasswordHash    string    `bun:"password_hash,notnull"`
	RecoveryKeyHash *string   `bun:"recovery_key_hash"`
	Role            string    `bun:"role,notnull"`
	QuotaBytes      int64     `bun:"quota_bytes,notnull,default:-1"`
	CreatedAt       time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

type StorageNode struct {
	bun.BaseModel `bun:"table:storage_nodes,alias:sn"`

	ID                  string    `bun:"id,pk"`
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
