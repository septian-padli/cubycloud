-- 016_user_profile_and_token_version.sql
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
UPDATE users SET display_name = username WHERE display_name = '';
