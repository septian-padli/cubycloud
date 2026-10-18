CREATE TABLE IF NOT EXISTS my_storage_items (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('file', 'folder')),
    size_bytes INTEGER NOT NULL DEFAULT 0,
    parent_id TEXT REFERENCES my_storage_items(id),
    disk_key TEXT,
    modified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_my_storage_owner_parent ON my_storage_items(owner_id, parent_id, deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_my_storage_sibling_name ON my_storage_items(owner_id, COALESCE(parent_id, ''), name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_my_storage_owner_disk ON my_storage_items(owner_id, disk_key);
