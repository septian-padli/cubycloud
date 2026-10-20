-- 008_create_organization_items.sql
CREATE TABLE IF NOT EXISTS organization_items (
    id TEXT PRIMARY KEY,
    created_by TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('file', 'folder')),
    size_bytes INTEGER NOT NULL DEFAULT 0,
    parent_id TEXT REFERENCES organization_items(id),
    disk_key TEXT,
    storage_node_id TEXT REFERENCES storage_nodes(id),
    modified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_org_items_parent ON organization_items(parent_id, deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_org_items_sibling_name ON organization_items(COALESCE(parent_id, ''), name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_org_items_disk ON organization_items(disk_key);
