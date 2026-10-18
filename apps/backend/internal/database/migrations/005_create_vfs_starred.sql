CREATE TABLE IF NOT EXISTS vfs_starred (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    starred_at DATETIME NOT NULL,
    UNIQUE(user_id, node_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (node_id) REFERENCES vfs_nodes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_vfs_starred_user ON vfs_starred(user_id, starred_at DESC);
