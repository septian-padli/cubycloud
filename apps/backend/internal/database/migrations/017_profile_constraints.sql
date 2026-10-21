CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users(LOWER(email));

UPDATE users SET display_name = username WHERE display_name = '';

CREATE TRIGGER IF NOT EXISTS users_display_name_default
AFTER INSERT ON users
WHEN NEW.display_name = ''
BEGIN
    UPDATE users SET display_name = NEW.username WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS users_email_case_insensitive_insert
BEFORE INSERT ON users
WHEN EXISTS (SELECT 1 FROM users WHERE LOWER(email) = LOWER(NEW.email))
BEGIN
    SELECT RAISE(ABORT, 'email already in use');
END;

CREATE TRIGGER IF NOT EXISTS users_email_case_insensitive_update
BEFORE UPDATE OF email ON users
WHEN EXISTS (SELECT 1 FROM users WHERE id != NEW.id AND LOWER(email) = LOWER(NEW.email))
BEGIN
    SELECT RAISE(ABORT, 'email already in use');
END;
