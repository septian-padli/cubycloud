-- 011_create_share_tables.sql
CREATE TABLE IF NOT EXISTS share_grants (
    id TEXT PRIMARY KEY,
    item_id TEXT NOT NULL REFERENCES my_storage_items(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK(permission IN ('viewer', 'editor')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(item_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_share_grants_item ON share_grants(item_id);
CREATE INDEX IF NOT EXISTS idx_share_grants_user ON share_grants(user_id);

CREATE TABLE IF NOT EXISTS share_links (
    id TEXT PRIMARY KEY,
    item_id TEXT NOT NULL REFERENCES my_storage_items(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    permission TEXT NOT NULL CHECK(permission IN ('viewer', 'editor')),
    password_hash TEXT,
    expires_at DATETIME,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_share_links_token ON share_links(token);
CREATE INDEX IF NOT EXISTS idx_share_links_item ON share_links(item_id);
CREATE INDEX IF NOT EXISTS idx_share_links_created_by ON share_links(created_by);
