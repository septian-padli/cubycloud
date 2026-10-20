-- 003_create_settings.sql
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Bootstrap defaults
INSERT OR IGNORE INTO settings (key, value) VALUES 
    ('is_setup_completed', 'false'),
    ('placement_strategy', 'FILL_FIRST'),
    ('jwt_secret_version', '1'),
    ('default_user_quota_gb', '-1'),
    ('worker_concurrency_limit', '2'),
    ('audit_log_retention_days', '90');
