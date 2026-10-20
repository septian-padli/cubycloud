-- 009_user_tombstone.sql
ALTER TABLE users ADD COLUMN deleted_at DATETIME;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
