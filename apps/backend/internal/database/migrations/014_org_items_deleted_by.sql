-- 014_org_items_deleted_by.sql
ALTER TABLE organization_items ADD COLUMN deleted_by TEXT REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_org_items_deleted ON organization_items(deleted_by, deleted_at);
