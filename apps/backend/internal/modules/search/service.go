package search

import (
	"context"
	"database/sql"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

type SearchItem struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Source    string  `json:"source"`
	SizeBytes int64   `json:"size_bytes"`
	MimeType  string  `json:"mime_type"`
	ParentID  *string `json:"parent_id"`
	Location  string  `json:"location"`
	UpdatedAt string  `json:"updated_at"`
}

type SearchResponse struct {
	Items  []SearchItem `json:"items"`
	Total  int64        `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

type Service struct {
	db *bun.DB
}

func NewService(db *bun.DB) *Service {
	return &Service{db: db}
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (s *Service) Search(ctx context.Context, userID, query string, limit, offset int) (SearchResponse, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return SearchResponse{
			Items:  make([]SearchItem, 0),
			Total:  0,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	likePattern := "%" + escapeLike(trimmed) + "%"

	cteQuery := `
WITH RECURSIVE shared_tree(id) AS (
    SELECT sg.item_id FROM share_grants sg
    JOIN my_storage_items r ON r.id = sg.item_id
    WHERE sg.user_id = ? AND r.deleted_at IS NULL
    UNION ALL
    SELECT m.id FROM my_storage_items m
    JOIN shared_tree st ON m.parent_id = st.id
    WHERE m.deleted_at IS NULL
),
search_results AS (
    SELECT id, name, kind AS type, 'my-storage' AS source, size_bytes, parent_id, modified_at
    FROM my_storage_items
    WHERE owner_id = ? AND deleted_at IS NULL AND name LIKE ? ESCAPE '\'

    UNION ALL

    SELECT id, name, kind AS type, 'organization' AS source, size_bytes, parent_id, modified_at
    FROM organization_items
    WHERE deleted_at IS NULL AND name LIKE ? ESCAPE '\'

    UNION ALL

    SELECT id, name, kind AS type, 'shared' AS source, size_bytes,
           CASE WHEN EXISTS (
               SELECT 1 FROM share_grants sg WHERE sg.item_id = my_storage_items.id AND sg.user_id = ?
           ) THEN NULL ELSE parent_id END AS parent_id, modified_at
    FROM my_storage_items
    WHERE owner_id != ? AND deleted_at IS NULL AND name LIKE ? ESCAPE '\'
      AND (
        id IN (SELECT id FROM shared_tree)
        OR parent_id IN (SELECT id FROM shared_tree)
      )
)
`

	// 1. Count total results
	countSQL := cteQuery + `SELECT COUNT(*) FROM search_results`
	var total int64
	err := s.db.QueryRowContext(ctx, countSQL,
		userID,              // shared_tree user_id
		userID, likePattern, // my-storage
		likePattern,                 // organization
		userID, userID, likePattern, // shared root parent masking + shared
	).Scan(&total)
	if err != nil {
		return SearchResponse{}, err
	}

	res := SearchResponse{
		Items:  make([]SearchItem, 0),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	if total == 0 || int64(offset) >= total {
		return res, nil
	}

	// 2. Fetch page results
	fetchSQL := cteQuery + `
SELECT id, name, type, source, size_bytes, parent_id, modified_at
FROM search_results
ORDER BY modified_at DESC, name COLLATE NOCASE ASC, source ASC, id ASC
LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, fetchSQL,
		userID,              // shared_tree user_id
		userID, likePattern, // my-storage
		likePattern,                 // organization
		userID, userID, likePattern, // shared root parent masking + shared
		limit, offset,
	)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	type rawItem struct {
		id         string
		name       string
		itemType   string
		source     string
		sizeBytes  int64
		parentID   *string
		modifiedAt time.Time
	}

	var rawItems []rawItem
	for rows.Next() {
		var it rawItem
		var parentID sql.NullString
		if err := rows.Scan(&it.id, &it.name, &it.itemType, &it.source, &it.sizeBytes, &parentID, &it.modifiedAt); err != nil {
			return res, err
		}
		if parentID.Valid {
			it.parentID = &parentID.String
		}
		rawItems = append(rawItems, it)
	}
	if err := rows.Err(); err != nil {
		return res, err
	}

	// 3. Resolve locations with cache
	locationCache := make(map[string]string)
	for _, it := range rawItems {
		loc := s.resolveLocation(ctx, userID, it.source, it.id, it.parentID, locationCache)
		mimeType := resolveMimeType(it.name, it.itemType)
		res.Items = append(res.Items, SearchItem{
			ID:        it.id,
			Name:      it.name,
			Type:      it.itemType,
			Source:    it.source,
			SizeBytes: it.sizeBytes,
			MimeType:  mimeType,
			ParentID:  it.parentID,
			Location:  loc,
			UpdatedAt: it.modifiedAt.UTC().Format(time.RFC3339),
		})
	}

	return res, nil
}

func (s *Service) resolveLocation(ctx context.Context, userID, source, itemID string, parentID *string, cache map[string]string) string {
	rootPrefix := "My Storage"
	table := "my_storage_items"
	if source == "organization" {
		rootPrefix = "Organization"
		table = "organization_items"
	} else if source == "shared" {
		rootPrefix = "Shared With Me"
	}
	if parentID == nil || *parentID == "" {
		return rootPrefix
	}

	cacheKey := source + ":" + *parentID
	if cached, ok := cache[cacheKey]; ok {
		return cached
	}

	if source == "shared" {
		var itemIsShared bool
		if err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM share_grants WHERE item_id = ? AND user_id = ?)`,
			itemID, userID,
		).Scan(&itemIsShared); err != nil {
			cache[cacheKey] = rootPrefix
			return rootPrefix
		}
		if itemIsShared {
			cache[cacheKey] = rootPrefix
			return rootPrefix
		}
	}

	var pathQuery string
	args := []any{*parentID}
	if source == "shared" {
		pathQuery = `
WITH RECURSIVE path_tree(id, name, parent_id, depth, is_share_root) AS (
    SELECT id, name, parent_id, 0,
           EXISTS(SELECT 1 FROM share_grants WHERE item_id = id AND user_id = ?)
    FROM my_storage_items WHERE id = ?
    UNION ALL
    SELECT t.id, t.name, t.parent_id, pt.depth + 1,
           EXISTS(SELECT 1 FROM share_grants WHERE item_id = t.id AND user_id = ?)
    FROM my_storage_items t
    JOIN path_tree pt ON t.id = pt.parent_id
    WHERE pt.is_share_root = 0
)
SELECT name FROM path_tree ORDER BY depth DESC`
		args = []any{userID, *parentID, userID}
	} else {
		pathQuery = `
WITH RECURSIVE path_tree(id, name, parent_id, depth) AS (
    SELECT id, name, parent_id, 0 FROM ` + table + ` WHERE id = ?
    UNION ALL
    SELECT t.id, t.name, t.parent_id, pt.depth + 1
    FROM ` + table + ` t
    JOIN path_tree pt ON t.id = pt.parent_id
)
SELECT name FROM path_tree ORDER BY depth DESC`
	}

	rows, err := s.db.QueryContext(ctx, pathQuery, args...)
	if err != nil {
		cache[cacheKey] = rootPrefix
		return rootPrefix
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			names = append(names, n)
		}
	}
	if rows.Err() != nil {
		cache[cacheKey] = rootPrefix
		return rootPrefix
	}

	result := rootPrefix
	if len(names) > 0 {
		result += " / " + strings.Join(names, " / ")
	}
	cache[cacheKey] = result
	return result
}

func resolveMimeType(name, kind string) string {
	if kind == "folder" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".markdown", ".csv", ".log", ".json", ".jsonl", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf", ".go", ".js", ".jsx", ".ts", ".tsx", ".svelte", ".css", ".scss", ".html", ".htm", ".py", ".rs", ".java", ".c", ".h", ".cpp", ".hpp", ".sh", ".sql":
		return "text/plain; charset=utf-8"
	}

	if mt := mime.TypeByExtension(ext); mt != "" {
		if idx := strings.Index(mt, ";"); idx != -1 {
			return strings.TrimSpace(mt[:idx])
		}
		return mt
	}
	return "application/octet-stream"
}
