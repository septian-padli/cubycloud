CREATE TABLE IF NOT EXISTS vfs_recent_access (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    source TEXT NOT NULL CHECK(source IN ('my-storage', 'organization')),
    accessed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, item_id, source),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_recent_access_user ON vfs_recent_access(user_id, accessed_at DESC);

CREATE TABLE IF NOT EXISTS activity_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('upload', 'edit')),
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_kind TEXT NOT NULL DEFAULT 'file',
    source TEXT NOT NULL CHECK(source IN ('my-storage', 'organization')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_activity_logs_user_time ON activity_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_logs_user_item ON activity_logs(user_id, source, item_id, created_at DESC);
