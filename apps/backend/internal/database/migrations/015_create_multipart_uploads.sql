-- 015_create_multipart_uploads.sql
-- Durable intent + part metadata for direct presigned S3 multipart uploads
-- (browser -> provider UploadPart). Mirrors the pending_cloud_uploads lifecycle:
-- a 24h durable intent so a browser reload can resume only absent/failed parts.
CREATE TABLE IF NOT EXISTS multipart_uploads (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL, -- 'my-storage' | 'organization'
    owner_id TEXT NOT NULL,
    storage_node_id TEXT NOT NULL,
    s3_key TEXT NOT NULL,
    s3_upload_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    part_size_bytes INTEGER NOT NULL,
    part_count INTEGER NOT NULL,
    parent_id TEXT,
    unique_name INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active', -- 'active' | 'completed'
    final_item_id TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    FOREIGN KEY (storage_node_id) REFERENCES storage_nodes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_multipart_uploads_owner ON multipart_uploads(owner_id);
CREATE INDEX IF NOT EXISTS idx_multipart_uploads_node ON multipart_uploads(storage_node_id);
CREATE INDEX IF NOT EXISTS idx_multipart_uploads_expires ON multipart_uploads(expires_at);

CREATE TABLE IF NOT EXISTS multipart_parts (
    upload_id TEXT NOT NULL REFERENCES multipart_uploads(id) ON DELETE CASCADE,
    part_number INTEGER NOT NULL,
    size_bytes INTEGER NOT NULL,
    etag TEXT,
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'completed' | 'failed'
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (upload_id, part_number)
);
CREATE INDEX IF NOT EXISTS idx_multipart_parts_upload ON multipart_parts(upload_id, status);
