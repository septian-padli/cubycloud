ALTER TABLE storage_nodes ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE my_storage_items ADD COLUMN storage_node_id TEXT REFERENCES storage_nodes(id);
CREATE INDEX IF NOT EXISTS idx_my_storage_storage_node ON my_storage_items(storage_node_id);
UPDATE storage_nodes SET name = CASE
    WHEN type = 'LOCAL' THEN ''
    ELSE COALESCE(NULLIF(bucket, ''), 'S3 storage')
END;
