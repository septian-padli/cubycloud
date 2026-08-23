-- 002_create_storage_nodes.sql
CREATE TABLE IF NOT EXISTS storage_nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL CHECK(type IN ('LOCAL', 'S3')),
    path TEXT,
    endpoint TEXT,
    bucket TEXT,
    region TEXT,
    access_key_encrypted TEXT,
    secret_key_encrypted TEXT,
    capacity_bytes INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_storage_nodes_enabled ON storage_nodes(enabled);
