-- 010_super_admin_unlimited_quota.sql
-- Keep stored quota values consistent with the Super Admin role contract.
UPDATE users
SET quota_mode = 'UNLIMITED', quota_bytes = -1
WHERE role = 'SUPER_ADMIN';
