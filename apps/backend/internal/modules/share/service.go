package share

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound                = errors.New("not_found")
	ErrForbidden               = errors.New("forbidden")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrExpired                  = errors.New("link_expired")
	ErrInvalidPassword         = errors.New("invalid_password")
	ErrInvalidPermission       = errors.New("invalid_permission")
	ErrInvalidTarget           = errors.New("invalid_target")
	ErrRootItemActionForbidden = errors.New("cannot perform this action on root shared item")
	ErrMoveOutsideSharedTree   = errors.New("cannot move outside shared folder tree")
	ErrReshareForbidden        = errors.New("only item owner can manage shares")
	ErrPasswordRequired        = errors.New("password required")
)

type Service struct {
	db        *bun.DB
	storage   storage.StorageDriver
	myStorage *mystorage.Service
}

func NewService(db *bun.DB, st storage.StorageDriver) *Service {
	return &Service{
		db:        db,
		storage:   st,
		myStorage: mystorage.NewService(db, st),
	}
}

// -------------------------------------------------------------------------
// 1. User Autocomplete
// -------------------------------------------------------------------------

func (s *Service) SearchUsers(ctx context.Context, currentUserID string, query string) ([]UserSummary, error) {
	query = strings.TrimSpace(query)
	var users []UserSummary

	q := s.db.NewSelect().
		TableExpr("users AS u").
		Column("u.id", "u.username", "u.email").
		Where("u.deleted_at IS NULL").
		Where("u.id != ?", currentUserID)

	if query != "" {
		likePattern := "%" + query + "%"
		q = q.Where("(u.username LIKE ? OR u.email LIKE ?)", likePattern, likePattern)
	}

	err := q.Order("u.username ASC").Limit(20).Scan(ctx, &users)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	if users == nil {
		users = []UserSummary{}
	}
	return users, nil
}

// -------------------------------------------------------------------------
// 2. Item Shares Management (Owner Only, Reshare Forbidden)
// -------------------------------------------------------------------------

// checkItemOwner verifies that item exists, not deleted, and currentUserID is owner.
func (s *Service) checkItemOwner(ctx context.Context, currentUserID, itemID string) error {
	var ownerID string
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id FROM my_storage_items WHERE id = ? AND deleted_at IS NULL`,
		itemID,
	).Scan(&ownerID)

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != currentUserID {
		return ErrReshareForbidden
	}
	return nil
}

func (s *Service) GetItemShares(ctx context.Context, ownerID, itemID string) (*ItemSharesResponse, error) {
	if err := s.checkItemOwner(ctx, ownerID, itemID); err != nil {
		return nil, err
	}

	// Fetch grants
	rows, err := s.db.QueryContext(ctx, `
		SELECT sg.id, sg.user_id, u.username, u.email, sg.permission
		FROM share_grants sg
		JOIN users u ON u.id = sg.user_id
		WHERE sg.item_id = ? AND u.deleted_at IS NULL
		ORDER BY sg.created_at ASC
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("query grants: %w", err)
	}
	defer rows.Close()

	grants := make([]GrantResponse, 0)
	for rows.Next() {
		var g GrantResponse
		if err := rows.Scan(&g.ID, &g.UserID, &g.Username, &g.Email, &g.Permission); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}

	// Fetch links
	linkRows, err := s.db.QueryContext(ctx, `
		SELECT id, token, permission, (password_hash IS NOT NULL AND password_hash != ''), expires_at, created_at
		FROM share_links
		WHERE item_id = ?
		ORDER BY created_at DESC
	`, itemID)
	if err != nil {
		return nil, fmt.Errorf("query links: %w", err)
	}
	defer linkRows.Close()

	links := make([]LinkResponse, 0)
	for linkRows.Next() {
		var l LinkResponse
		if err := linkRows.Scan(&l.ID, &l.Token, &l.Permission, &l.HasPassword, &l.ExpiresAt, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.URL = "/s/" + l.Token
		links = append(links, l)
	}

	return &ItemSharesResponse{
		Grants: grants,
		Links:  links,
	}, nil
}

func (s *Service) PutUserGrants(ctx context.Context, ownerID, itemID string, inputs []GrantInput) error {
	if err := s.checkItemOwner(ctx, ownerID, itemID); err != nil {
		return err
	}

	// Validate inputs
	for _, in := range inputs {
		if in.Permission != "viewer" && in.Permission != "editor" {
			return ErrInvalidPermission
		}
		if in.UserID == ownerID {
			return errors.New("cannot share with oneself")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing grants for this item
	if _, err := tx.ExecContext(ctx, `DELETE FROM share_grants WHERE item_id = ?`, itemID); err != nil {
		return err
	}

	now := time.Now()
	for _, in := range inputs {
		grantID := ulid.Make().String()
		_, err := tx.ExecContext(ctx, `
			INSERT INTO share_grants (id, item_id, user_id, permission, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, grantID, itemID, in.UserID, in.Permission, now, now)
		if err != nil {
			return fmt.Errorf("insert grant: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Service) DeleteUserGrant(ctx context.Context, ownerID, itemID, targetUserID string) error {
	if err := s.checkItemOwner(ctx, ownerID, itemID); err != nil {
		return err
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM share_grants WHERE item_id = ? AND user_id = ?`, itemID, targetUserID)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) CreateShareLink(ctx context.Context, ownerID, itemID, permission, password string, expiresDays *int) (*LinkResponse, error) {
	if err := s.checkItemOwner(ctx, ownerID, itemID); err != nil {
		return nil, err
	}
	if permission != "viewer" && permission != "editor" {
		return nil, ErrInvalidPermission
	}

	var passwordHash *string
	if password = strings.TrimSpace(password); password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		strHash := string(hash)
		passwordHash = &strHash
	}

	var expiresAt *time.Time
	if expiresDays != nil && *expiresDays > 0 {
		t := time.Now().Add(time.Duration(*expiresDays) * 24 * time.Hour)
		expiresAt = &t
	}

	token, err := GenerateToken(12)
	if err != nil {
		return nil, err
	}

	linkID := ulid.Make().String()
	now := time.Now()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO share_links (id, item_id, token, permission, password_hash, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, linkID, itemID, token, permission, passwordHash, expiresAt, ownerID, now)
	if err != nil {
		return nil, fmt.Errorf("insert share link: %w", err)
	}

	return &LinkResponse{
		ID:          linkID,
		Token:       token,
		Permission:  permission,
		HasPassword: passwordHash != nil,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		URL:         "/s/" + token,
	}, nil
}

func (s *Service) DeleteShareLink(ctx context.Context, ownerID, itemID, linkID string) error {
	if err := s.checkItemOwner(ctx, ownerID, itemID); err != nil {
		return err
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM share_links WHERE id = ? AND item_id = ?`, linkID, itemID)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// -------------------------------------------------------------------------
// 3. Shared With Me (Internal User Shared Items)
// -------------------------------------------------------------------------

func (s *Service) ListSharedWithMe(ctx context.Context, currentUserID string) ([]SharedItemResponse, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.name, i.kind, i.size_bytes, i.modified_at, i.owner_id, u.username, sg.permission
		FROM share_grants sg
		JOIN my_storage_items i ON i.id = sg.item_id
		JOIN users u ON u.id = i.owner_id
		WHERE sg.user_id = ? AND i.deleted_at IS NULL
		ORDER BY i.kind ASC, i.name COLLATE NOCASE ASC
	`, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("list shared: %w", err)
	}
	defer rows.Close()

	items := make([]SharedItemResponse, 0)
	for rows.Next() {
		var item SharedItemResponse
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Kind, &item.SizeBytes,
			&item.ModifiedAt, &item.OwnerID, &item.OwnerName, &item.Permission,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

type itemAccessInfo struct {
	RootSharedID string
	Permission   string
	OwnerID      string
}

// resolveSharedAccess finds if currentUserID has access to itemID either directly or via an ancestor share_grant.
func (s *Service) resolveSharedAccess(ctx context.Context, currentUserID, itemID string) (*itemAccessInfo, error) {
	var info itemAccessInfo
	err := s.db.QueryRowContext(ctx, `
		WITH RECURSIVE ancestors(id, parent_id, depth) AS (
			SELECT id, parent_id, 0 FROM my_storage_items WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT m.id, m.parent_id, a.depth + 1
			FROM my_storage_items m
			JOIN ancestors a ON m.id = a.parent_id
			WHERE m.deleted_at IS NULL
		)
		SELECT a.id, sg.permission, (SELECT owner_id FROM my_storage_items WHERE id = ?)
		FROM ancestors a
		JOIN share_grants sg ON sg.item_id = a.id
		WHERE sg.user_id = ?
		ORDER BY a.depth DESC
		LIMIT 1
	`, itemID, itemID, currentUserID).Scan(&info.RootSharedID, &info.Permission, &info.OwnerID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// isDescendant checks if childID is equal to or a descendant of rootID (within non-deleted items).
func (s *Service) isDescendant(ctx context.Context, rootID, childID string) (bool, error) {
	if rootID == childID {
		return true, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
		WITH RECURSIVE ancestors(id, parent_id) AS (
			SELECT id, parent_id FROM my_storage_items WHERE id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT m.id, m.parent_id
			FROM my_storage_items m
			JOIN ancestors a ON m.id = a.parent_id
			WHERE m.deleted_at IS NULL
		)
		SELECT COUNT(*) FROM ancestors WHERE id = ?
	`, childID, rootID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Service) GetSharedFolder(ctx context.Context, currentUserID, folderID string) (*SharedFolderViewResponse, error) {
	access, err := s.resolveSharedAccess(ctx, currentUserID, folderID)
	if err != nil {
		return nil, err
	}

	// Verify target is folder
	var kind string
	err = s.db.QueryRowContext(ctx, `SELECT kind FROM my_storage_items WHERE id = ? AND deleted_at IS NULL`, folderID).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if kind != "folder" {
		return nil, errors.New("item is not a folder")
	}

	// Get items inside folderID
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, kind, size_bytes, parent_id, modified_at
		FROM my_storage_items
		WHERE parent_id = ? AND deleted_at IS NULL
		ORDER BY kind ASC, name COLLATE NOCASE ASC
	`, folderID)
	if err != nil {
		return nil, fmt.Errorf("list folder items: %w", err)
	}
	defer rows.Close()

	items := make([]SharedNode, 0)
	for rows.Next() {
		var n SharedNode
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.SizeBytes, &n.ParentID, &n.ModifiedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}

	// Build breadcrumbs from rootSharedID down to folderID
	breadcrumbs, err := s.buildBreadcrumbs(ctx, access.RootSharedID, folderID)
	if err != nil {
		return nil, err
	}

	return &SharedFolderViewResponse{
		RootID:      access.RootSharedID,
		Permission:  access.Permission,
		Items:       items,
		Breadcrumbs: breadcrumbs,
	}, nil
}

func (s *Service) buildBreadcrumbs(ctx context.Context, rootID, currentID string) ([]Breadcrumb, error) {
	// Query path from currentID up to rootID
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE ancestors(id, name, parent_id, depth) AS (
			SELECT id, name, parent_id, 0 FROM my_storage_items WHERE id = ?
			UNION ALL
			SELECT m.id, m.name, m.parent_id, a.depth + 1
			FROM my_storage_items m
			JOIN ancestors a ON m.id = a.parent_id
		)
		SELECT id, name FROM ancestors
		WHERE depth <= (SELECT depth FROM ancestors WHERE id = ?)
		ORDER BY depth DESC
	`, currentID, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	crumbs := make([]Breadcrumb, 0)
	for rows.Next() {
		var b Breadcrumb
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		crumbs = append(crumbs, b)
	}
	return crumbs, nil
}

func (s *Service) CreateSharedFolder(ctx context.Context, currentUserID, parentID, name string) (*SharedNode, error) {
	access, err := s.resolveSharedAccess(ctx, currentUserID, parentID)
	if err != nil {
		return nil, err
	}
	if access.Permission != "editor" {
		return nil, ErrForbidden
	}

	item, _, err := s.myStorage.CreateFolder(ctx, access.OwnerID, name, &parentID, false)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       item.Kind,
		SizeBytes:  item.SizeBytes,
		ParentID:   item.ParentID,
		ModifiedAt: item.ModifiedAt,
	}, nil
}

func (s *Service) UploadSharedFile(
	ctx context.Context,
	currentUserID, parentID, filename string,
	r io.Reader,
	uniqueName bool,
	maxBytes int64,
	limitErr error,
) (*SharedNode, error) {
	access, err := s.resolveSharedAccess(ctx, currentUserID, parentID)
	if err != nil {
		return nil, err
	}
	if access.Permission != "editor" {
		return nil, ErrForbidden
	}

	item, err := s.myStorage.Upload(ctx, access.OwnerID, filename, &parentID, r, uniqueName, maxBytes, limitErr)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       item.Kind,
		SizeBytes:  item.SizeBytes,
		ParentID:   item.ParentID,
		ModifiedAt: item.ModifiedAt,
	}, nil
}

func (s *Service) RenameSharedItem(ctx context.Context, currentUserID, itemID, newName string) (*SharedNode, error) {
	access, err := s.resolveSharedAccess(ctx, currentUserID, itemID)
	if err != nil {
		return nil, err
	}
	if access.Permission != "editor" {
		return nil, ErrForbidden
	}
	if itemID == access.RootSharedID {
		return nil, ErrRootItemActionForbidden
	}

	item, _, err := s.myStorage.Update(ctx, access.OwnerID, itemID, &newName, nil, false)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       item.Kind,
		SizeBytes:  item.SizeBytes,
		ParentID:   item.ParentID,
		ModifiedAt: item.ModifiedAt,
	}, nil
}

func (s *Service) DeleteSharedItem(ctx context.Context, currentUserID, itemID string) error {
	access, err := s.resolveSharedAccess(ctx, currentUserID, itemID)
	if err != nil {
		return err
	}
	if access.Permission != "editor" {
		return ErrForbidden
	}
	if itemID == access.RootSharedID {
		return ErrRootItemActionForbidden
	}

	return s.myStorage.Delete(ctx, access.OwnerID, itemID)
}

func (s *Service) MoveSharedItem(ctx context.Context, currentUserID, itemID string, targetParentID *string) (*SharedNode, error) {
	if targetParentID == nil || *targetParentID == "" {
		return nil, ErrMoveOutsideSharedTree
	}
	access, err := s.resolveSharedAccess(ctx, currentUserID, itemID)
	if err != nil {
		return nil, err
	}
	if access.Permission != "editor" {
		return nil, ErrForbidden
	}
	if itemID == access.RootSharedID {
		return nil, ErrRootItemActionForbidden
	}

	// Verify target parent is within the shared tree
	isTargetInside, err := s.isDescendant(ctx, access.RootSharedID, *targetParentID)
	if err != nil {
		return nil, err
	}
	if !isTargetInside {
		return nil, ErrMoveOutsideSharedTree
	}

	node, _, err := s.myStorage.MoveItem(ctx, access.OwnerID, itemID, targetParentID)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         node.ID,
		Name:       node.Name,
		Kind:       node.Kind,
		SizeBytes:  node.SizeBytes,
		ParentID:   node.ParentID,
		ModifiedAt: node.ModifiedAt,
	}, nil
}

func (s *Service) DownloadSharedItem(ctx context.Context, currentUserID, itemID string) (mystorage.Item, io.ReadCloser, error) {
	access, err := s.resolveSharedAccess(ctx, currentUserID, itemID)
	if err != nil {
		return mystorage.Item{}, nil, err
	}
	return s.myStorage.Download(ctx, access.OwnerID, itemID)
}

func (s *Service) UploadLimit(ctx context.Context, ownerID string) (int64, error, error) {
	return s.myStorage.UploadLimit(ctx, ownerID)
}

// -------------------------------------------------------------------------
// 4. Public Share Links
// -------------------------------------------------------------------------

func (s *Service) lookupLink(ctx context.Context, token string) (*ShareLink, *mystorage.Item, error) {
	var link ShareLink
	err := s.db.NewSelect().Model(&link).Where("token = ?", token).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	// Check expiry
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, nil, ErrExpired
	}

	// Check item exists and not deleted
	var item mystorage.Item
	err = s.db.QueryRowContext(ctx, `
		SELECT id, name, kind, size_bytes, parent_id, modified_at, disk_key
		FROM my_storage_items
		WHERE id = ? AND deleted_at IS NULL
	`, link.ItemID).Scan(&item.ID, &item.Name, &item.Kind, &item.SizeBytes, &item.ParentID, &item.ModifiedAt, &item.DiskKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	return &link, &item, nil
}

func (s *Service) GetPublicShare(ctx context.Context, token string, unlocked bool, folderID string) (*PublicShareResponse, error) {
	link, rootItem, err := s.lookupLink(ctx, token)
	if err != nil {
		return nil, err
	}

	hasPassword := link.PasswordHash != nil && *link.PasswordHash != ""

	if hasPassword && !unlocked {
		return &PublicShareResponse{
			Token:       token,
			Permission:  link.Permission,
			HasPassword: true,
			Unlocked:    false,
			ItemName:    rootItem.Name,
		}, nil
	}

	// Unlocked or no password
	targetItem := rootItem
	if folderID != "" && folderID != rootItem.ID {
		isSub, err := s.isDescendant(ctx, rootItem.ID, folderID)
		if err != nil || !isSub {
			return nil, ErrNotFound
		}
		var sub mystorage.Item
		err = s.db.QueryRowContext(ctx, `
			SELECT id, name, kind, size_bytes, parent_id, modified_at, disk_key
			FROM my_storage_items
			WHERE id = ? AND deleted_at IS NULL
		`, folderID).Scan(&sub.ID, &sub.Name, &sub.Kind, &sub.SizeBytes, &sub.ParentID, &sub.ModifiedAt, &sub.DiskKey)
		if err != nil {
			return nil, ErrNotFound
		}
		targetItem = &sub
	}

	var children []SharedNode
	if targetItem.Kind == "folder" {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, name, kind, size_bytes, parent_id, modified_at
			FROM my_storage_items
			WHERE parent_id = ? AND deleted_at IS NULL
			ORDER BY kind ASC, name COLLATE NOCASE ASC
		`, targetItem.ID)
		if err == nil {
			defer rows.Close()
			children = make([]SharedNode, 0)
			for rows.Next() {
				var c SharedNode
				if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &c.SizeBytes, &c.ParentID, &c.ModifiedAt); err == nil {
					children = append(children, c)
				}
			}
		}
	}

	breadcrumbs, _ := s.buildBreadcrumbs(ctx, rootItem.ID, targetItem.ID)

	return &PublicShareResponse{
		Token:       token,
		Permission:  link.Permission,
		HasPassword: hasPassword,
		Unlocked:    true,
		ExpiresAt:   link.ExpiresAt,
		Item: &SharedNode{
			ID:         targetItem.ID,
			Name:       targetItem.Name,
			Kind:       targetItem.Kind,
			SizeBytes:  targetItem.SizeBytes,
			ParentID:   targetItem.ParentID,
			ModifiedAt: targetItem.ModifiedAt,
		},
		Children:    children,
		Breadcrumbs: breadcrumbs,
	}, nil
}

func (s *Service) UnlockPublicShare(ctx context.Context, token, password string) (*ShareLink, error) {
	link, _, err := s.lookupLink(ctx, token)
	if err != nil {
		return nil, err
	}

	if link.PasswordHash != nil && *link.PasswordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(*link.PasswordHash), []byte(password)); err != nil {
			return nil, ErrInvalidPassword
		}
	}
	return link, nil
}

func (s *Service) DownloadPublicFile(ctx context.Context, token string, unlocked bool, fileID string) (mystorage.Item, io.ReadCloser, error) {
	link, rootItem, err := s.lookupLink(ctx, token)
	if err != nil {
		return mystorage.Item{}, nil, err
	}

	if link.PasswordHash != nil && *link.PasswordHash != "" && !unlocked {
		return mystorage.Item{}, nil, ErrPasswordRequired
	}

	// If root item is a file, download directly
	if rootItem.Kind == "file" {
		return s.myStorage.Download(ctx, link.CreatedBy, rootItem.ID)
	}

	// Root item is folder
	if fileID != "" && fileID != rootItem.ID {
		isSub, err := s.isDescendant(ctx, rootItem.ID, fileID)
		if err != nil || !isSub {
			return mystorage.Item{}, nil, ErrNotFound
		}
		return s.myStorage.Download(ctx, link.CreatedBy, fileID)
	}

	// Download root folder zip
	return s.myStorage.Download(ctx, link.CreatedBy, rootItem.ID)
}

func (s *Service) DownloadPublicZip(ctx context.Context, token string, unlocked bool, folderID string) (mystorage.Item, io.ReadCloser, error) {
	link, rootItem, err := s.lookupLink(ctx, token)
	if err != nil {
		return mystorage.Item{}, nil, err
	}

	if link.PasswordHash != nil && *link.PasswordHash != "" && !unlocked {
		return mystorage.Item{}, nil, ErrPasswordRequired
	}

	targetID := rootItem.ID
	if folderID != "" && folderID != rootItem.ID {
		isSub, err := s.isDescendant(ctx, rootItem.ID, folderID)
		if err != nil || !isSub {
			return mystorage.Item{}, nil, ErrNotFound
		}
		targetID = folderID
	}

	return s.myStorage.Download(ctx, link.CreatedBy, targetID)
}

// Public link editor write actions (require authenticated user and editor permission)

func (s *Service) verifyPublicEditor(ctx context.Context, token string, unlocked bool) (*ShareLink, *mystorage.Item, error) {
	link, rootItem, err := s.lookupLink(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	if link.PasswordHash != nil && *link.PasswordHash != "" && !unlocked {
		return nil, nil, ErrPasswordRequired
	}

	if link.Permission != "editor" {
		return nil, nil, ErrForbidden
	}

	return link, rootItem, nil
}

func (s *Service) UploadPublicFile(
	ctx context.Context,
	token string,
	unlocked bool,
	parentID *string,
	filename string,
	r io.Reader,
	uniqueName bool,
	maxBytes int64,
	limitErr error,
) (*SharedNode, error) {
	link, rootItem, err := s.verifyPublicEditor(ctx, token, unlocked)
	if err != nil {
		return nil, err
	}

	targetParent := rootItem.ID
	if parentID != nil && *parentID != "" {
		isSub, err := s.isDescendant(ctx, rootItem.ID, *parentID)
		if err != nil || !isSub {
			return nil, ErrNotFound
		}
		targetParent = *parentID
	}

	item, err := s.myStorage.Upload(ctx, link.CreatedBy, filename, &targetParent, r, uniqueName, maxBytes, limitErr)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       item.Kind,
		SizeBytes:  item.SizeBytes,
		ParentID:   item.ParentID,
		ModifiedAt: item.ModifiedAt,
	}, nil
}

func (s *Service) CreatePublicFolder(ctx context.Context, token string, unlocked bool, parentID *string, name string) (*SharedNode, error) {
	link, rootItem, err := s.verifyPublicEditor(ctx, token, unlocked)
	if err != nil {
		return nil, err
	}

	targetParent := rootItem.ID
	if parentID != nil && *parentID != "" {
		isSub, err := s.isDescendant(ctx, rootItem.ID, *parentID)
		if err != nil || !isSub {
			return nil, ErrNotFound
		}
		targetParent = *parentID
	}

	item, _, err := s.myStorage.CreateFolder(ctx, link.CreatedBy, name, &targetParent, false)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       item.Kind,
		SizeBytes:  item.SizeBytes,
		ParentID:   item.ParentID,
		ModifiedAt: item.ModifiedAt,
	}, nil
}

func (s *Service) RenamePublicItem(ctx context.Context, token string, unlocked bool, itemID, newName string) (*SharedNode, error) {
	link, rootItem, err := s.verifyPublicEditor(ctx, token, unlocked)
	if err != nil {
		return nil, err
	}

	if itemID == rootItem.ID {
		return nil, ErrRootItemActionForbidden
	}

	isSub, err := s.isDescendant(ctx, rootItem.ID, itemID)
	if err != nil || !isSub {
		return nil, ErrNotFound
	}

	item, _, err := s.myStorage.Update(ctx, link.CreatedBy, itemID, &newName, nil, false)
	if err != nil {
		return nil, err
	}
	return &SharedNode{
		ID:         item.ID,
		Name:       item.Name,
		Kind:       item.Kind,
		SizeBytes:  item.SizeBytes,
		ParentID:   item.ParentID,
		ModifiedAt: item.ModifiedAt,
	}, nil
}

func (s *Service) DeletePublicItem(ctx context.Context, token string, unlocked bool, itemID string) error {
	link, rootItem, err := s.verifyPublicEditor(ctx, token, unlocked)
	if err != nil {
		return err
	}

	if itemID == rootItem.ID {
		return ErrRootItemActionForbidden
	}

	isSub, err := s.isDescendant(ctx, rootItem.ID, itemID)
	if err != nil || !isSub {
		return ErrNotFound
	}

	return s.myStorage.Delete(ctx, link.CreatedBy, itemID)
}

func CleanFilename(name string) string {
	return filepath.Base(strings.ReplaceAll(name, "\\", "/"))
}
