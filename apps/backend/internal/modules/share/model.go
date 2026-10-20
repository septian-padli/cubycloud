package share

import (
	"time"

	"github.com/uptrace/bun"
)

// ShareGrant represents a user-to-user share grant on a My Storage item.
type ShareGrant struct {
	bun.BaseModel `bun:"table:share_grants,alias:sg"`

	ID         string    `bun:"id,pk" json:"id"`
	ItemID     string    `bun:"item_id,notnull" json:"item_id"`
	UserID     string    `bun:"user_id,notnull" json:"user_id"`
	Permission string    `bun:"permission,notnull" json:"permission"` // 'viewer' | 'editor'
	CreatedAt  time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt  time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
}

// ShareLink represents a public / password-protected shareable link.
type ShareLink struct {
	bun.BaseModel `bun:"table:share_links,alias:sl"`

	ID           string     `bun:"id,pk" json:"id"`
	ItemID       string     `bun:"item_id,notnull" json:"item_id"`
	Token        string     `bun:"token,notnull,unique" json:"token"`
	Permission   string     `bun:"permission,notnull" json:"permission"` // 'viewer' | 'editor'
	PasswordHash *string    `bun:"password_hash" json:"-"`
	ExpiresAt    *time.Time `bun:"expires_at" json:"expires_at"`
	CreatedBy    string     `bun:"created_by,notnull" json:"created_by"`
	CreatedAt    time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// UserSummary for autocomplete search
type UserSummary struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// GrantResponse for item shares list
type GrantResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Permission string `json:"permission"`
}

// LinkResponse for item shares list & create link response
type LinkResponse struct {
	ID          string     `json:"id"`
	Token       string     `json:"token"`
	Permission  string     `json:"permission"`
	HasPassword bool       `json:"has_password"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	URL         string     `json:"url,omitempty"`
}

// ItemSharesResponse combines grants and links for an item
type ItemSharesResponse struct {
	Grants []GrantResponse `json:"grants"`
	Links  []LinkResponse  `json:"links"`
}

// GrantInput represents a single grant entry in a PUT request
type GrantInput struct {
	UserID     string `json:"user_id"`
	Permission string `json:"permission"` // 'viewer' | 'editor'
}

// PutGrantsRequest for updating user grants on an item
type PutGrantsRequest struct {
	Grants []GrantInput `json:"grants"`
}

// CreateLinkRequest for creating a new share link
type CreateLinkRequest struct {
	Permission  string `json:"permission"`             // 'viewer' | 'editor'
	Password    string `json:"password,omitempty"`     // plaintext password to be hashed
	ExpiresDays *int   `json:"expires_days,omitempty"` // 1, 7, 30, 90, or nil for never
}

// SharedItemResponse for listing items shared with the current user
type SharedItemResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	OwnerID    string    `json:"owner_id"`
	OwnerName  string    `json:"owner_name"`
	Permission string    `json:"permission"`
}

// SharedNode represents an item inside a shared folder view
type SharedNode struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	SizeBytes  int64     `json:"size_bytes"`
	ParentID   *string   `json:"parent_id"`
	ModifiedAt time.Time `json:"modified_at"`
}

// Breadcrumb represents a step in navigation
type Breadcrumb struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SharedFolderViewResponse for viewing a shared folder and its contents
type SharedFolderViewResponse struct {
	RootID      string       `json:"root_id"`
	Permission  string       `json:"permission"`
	Items       []SharedNode `json:"items"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
}

// PublicShareResponse for GET /api/v1/s/:token
type PublicShareResponse struct {
	Token       string       `json:"token"`
	Permission  string       `json:"permission"`
	HasPassword bool         `json:"has_password"`
	Unlocked    bool         `json:"unlocked"`
	ItemName    string       `json:"item_name,omitempty"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Item        *SharedNode  `json:"item,omitempty"`
	Children    []SharedNode `json:"children,omitempty"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs,omitempty"`
}

// PublicUnlockRequest for POST /api/v1/s/:token/unlock
type PublicUnlockRequest struct {
	Password string `json:"password"`
}

// CreateFolderRequest for creating a subfolder
type CreateFolderRequest struct {
	Name string `json:"name"`
}

// RenameItemRequest for renaming a shared item
type RenameItemRequest struct {
	Name string `json:"name"`
}

// MoveItemRequest for moving a shared item within the shared tree
type MoveItemRequest struct {
	TargetParentID *string `json:"target_parent_id"`
}
