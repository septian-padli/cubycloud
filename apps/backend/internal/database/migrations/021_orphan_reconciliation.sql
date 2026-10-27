CREATE TABLE IF NOT EXISTS orphan_candidates (
    storage_node_id TEXT NOT NULL,
    disk_key TEXT NOT NULL,
    first_seen_at TIMESTAMP NOT NULL,
    last_seen_at TIMESTAMP NOT NULL,
    observation_count INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (storage_node_id, disk_key)
);
