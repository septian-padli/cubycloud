package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigLoad_Defaults(t *testing.T) {
	// Clear any env
	t.Setenv("PORT", "")
	t.Setenv("DATA_DIR_PATH", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ENV", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("ALLOWED_ORIGINS", "")

	cfg := Load()
	assert.Equal(t, "8383", cfg.Port)
	assert.NotEmpty(t, cfg.DataDirPath)
	assert.NotEmpty(t, cfg.JWTSecret)
	assert.False(t, cfg.IsProduction)
	assert.False(t, cfg.CookieSecure)
	assert.Equal(t, []string{"*"}, cfg.AllowedOrigins)
	assert.Equal(t, filepath.Join(cfg.DataDirPath, "db", "cubycloud.db"), cfg.DBPath)
	assert.Equal(t, filepath.Join(cfg.DataDirPath, "storage"), cfg.StoragePath)
	assert.Equal(t, filepath.Join(cfg.DataDirPath, "temp", "tus"), cfg.TempTUSPath)
}
func TestConfigLoad_CustomEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PORT", "9090")
	t.Setenv("DATA_DIR_PATH", tmpDir)
	t.Setenv("JWT_SECRET", "my-custom-secret")

	cfg := Load()
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, tmpDir, cfg.DataDirPath)
	assert.Equal(t, "my-custom-secret", cfg.JWTSecret)
	assert.Equal(t, filepath.Join(tmpDir, "db", "cubycloud.db"), cfg.DBPath)
}
func TestConfigLoad_CustomOriginsAndCookie(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://app.cubycloud.local, https://drive.cubycloud.local")
	t.Setenv("COOKIE_SECURE", "true")

	cfg, err := LoadConfig()
	assert.NoError(t, err)
	assert.True(t, cfg.CookieSecure)
	assert.Equal(t, []string{"https://app.cubycloud.local", "https://drive.cubycloud.local"}, cfg.AllowedOrigins)
}

func TestConfigLoad_ProductionValidation(t *testing.T) {
	t.Run("auto-generates and persists JWT_SECRET in production on clean install", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "")

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.NotEmpty(t, cfg.JWTSecret)
		assert.GreaterOrEqual(t, len(cfg.JWTSecret), 32)

		secretFile := filepath.Join(tmpDir, "secrets", "jwt.key")
		raw, err := os.ReadFile(secretFile)
		assert.NoError(t, err)
		assert.Equal(t, cfg.JWTSecret, strings.TrimSpace(string(raw)))

		// Second load reuses the exact same secret
		cfg2, err := LoadConfig()
		assert.NoError(t, err)
		assert.Equal(t, cfg.JWTSecret, cfg2.JWTSecret)
	})

	t.Run("fails when JWT_SECRET missing while existing database is present", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "")

		// Simulate existing DB without secret file
		dbDir := filepath.Join(tmpDir, "db")
		assert.NoError(t, os.MkdirAll(dbDir, 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(dbDir, "cubycloud.db"), []byte("sqlite-data"), 0644))

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "JWT secret is missing while existing database is present")
	})

	t.Run("fails when persisted JWT secret is corrupted or too short", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "")

		secretsDir := filepath.Join(tmpDir, "secrets")
		assert.NoError(t, os.MkdirAll(secretsDir, 0700))
		assert.NoError(t, os.WriteFile(filepath.Join(secretsDir, "jwt.key"), []byte("too-short"), 0600))

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "persisted JWT secret in secrets/jwt.key is invalid")
	})

	t.Run("succeeds when COOKIE_SECURE is missing or false in production", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "strong-prod-secret-minimum-32-chars-long")
		t.Setenv("COOKIE_SECURE", "false")

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.False(t, cfg.CookieSecure)
	})

	t.Run("succeeds when ALLOWED_ORIGINS is missing in production with empty slice", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "strong-prod-secret-minimum-32-chars-long")
		t.Setenv("ALLOWED_ORIGINS", "")

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.Empty(t, cfg.AllowedOrigins)
	})

	t.Run("fails when ALLOWED_ORIGINS contains wildcard in production", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "strong-prod-secret-minimum-32-chars-long")
		t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, *")

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wildcard '*'")

		assert.Panics(t, func() {
			_ = Load()
		})
	})

	t.Run("succeeds with valid production env and trust proxy", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("ENV", "production")
		t.Setenv("DATA_DIR_PATH", tmpDir)
		t.Setenv("JWT_SECRET", "strong-prod-secret-minimum-32-chars-long")
		t.Setenv("COOKIE_SECURE", "true")
		t.Setenv("TRUST_PROXY", "true")
		t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, https://drive.example.com")

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.True(t, cfg.IsProduction)
		assert.True(t, cfg.CookieSecure)
		assert.True(t, cfg.TrustProxy)
		assert.Equal(t, []string{"https://app.example.com", "https://drive.example.com"}, cfg.AllowedOrigins)
	})
}

func TestParseDotEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	content := `
# Comment line
TEST_CUBY_KEY1=value1
TEST_CUBY_KEY2="quoted_value"
TEST_CUBY_KEY3='single_quoted'
INVALID_LINE_NO_EQUALS
`
	err := os.WriteFile(envPath, []byte(content), 0600)
	assert.NoError(t, err)

	ok := parseDotEnvFile(envPath)
	assert.True(t, ok)
	assert.Equal(t, "value1", os.Getenv("TEST_CUBY_KEY1"))
	assert.Equal(t, "quoted_value", os.Getenv("TEST_CUBY_KEY2"))
	assert.Equal(t, "single_quoted", os.Getenv("TEST_CUBY_KEY3"))
}
