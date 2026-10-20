-- 007_user_quota_mode.sql
ALTER TABLE users ADD COLUMN quota_mode TEXT NOT NULL DEFAULT 'DEFAULT';
UPDATE users SET quota_mode = 'UNLIMITED' WHERE role = 'SUPER_ADMIN' OR quota_bytes < 0;
UPDATE users SET quota_mode = 'CUSTOM' WHERE role != 'SUPER_ADMIN' AND quota_bytes >= 0;
