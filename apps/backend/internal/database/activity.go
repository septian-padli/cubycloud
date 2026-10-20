package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/oklog/ulid/v2"
)

type QueryExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// RecordActivity inserts an activity log entry for upload or edit.
func RecordActivity(ctx context.Context, q QueryExecer, userID, action, itemID, itemName, itemKind, source string) error {
	id := ulid.Make().String()
	_, err := q.ExecContext(ctx,
		`INSERT INTO activity_logs (id, user_id, action, item_id, item_name, item_kind, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, action, itemID, itemName, itemKind, source, time.Now().UTC(),
	)
	return err
}

// RecordRecentAccess records or updates the last accessed time of an item for a specific user.
func RecordRecentAccess(ctx context.Context, q QueryExecer, userID, itemID, source string) error {
	id := ulid.Make().String()
	now := time.Now().UTC()
	_, err := q.ExecContext(ctx,
		`INSERT INTO vfs_recent_access (id, user_id, item_id, source, accessed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, item_id, source) DO UPDATE SET accessed_at = excluded.accessed_at`,
		id, userID, itemID, source, now,
	)
	return err
}
