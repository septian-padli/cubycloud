CREATE TABLE IF NOT EXISTS pending_cloud_uploads (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    storage_node_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    parent_id TEXT,
    unique_name INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY (storage_node_id) REFERENCES storage_nodes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_pending_cloud_uploads_node ON pending_cloud_uploads(storage_node_id);
CREATE INDEX IF NOT EXISTS idx_pending_cloud_uploads_owner ON pending_cloud_uploads(owner_id);
