package config

import (
	"os"
	"path/filepath"
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
	t.Run("fails when JWT_SECRET missing in production", func(t *testing.T) {
		t.Setenv("ENV", "production")
		t.Setenv("JWT_SECRET", "")
		t.Setenv("COOKIE_SECURE", "true")
		t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "JWT_SECRET required")

		assert.Panics(t, func() {
			_ = Load()
		})
	})

	t.Run("fails when COOKIE_SECURE is missing or false in production", func(t *testing.T) {
		t.Setenv("ENV", "production")
		t.Setenv("JWT_SECRET", "strong-prod-secret")
		t.Setenv("COOKIE_SECURE", "false")
		t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "COOKIE_SECURE=true required")

		assert.Panics(t, func() {
			_ = Load()
		})
	})

	t.Run("fails when ALLOWED_ORIGINS is missing in production", func(t *testing.T) {
		t.Setenv("ENV", "production")
		t.Setenv("JWT_SECRET", "strong-prod-secret")
		t.Setenv("COOKIE_SECURE", "true")
		t.Setenv("ALLOWED_ORIGINS", "")

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ALLOWED_ORIGINS required")

		assert.Panics(t, func() {
			_ = Load()
		})
	})

	t.Run("fails when ALLOWED_ORIGINS contains wildcard in production", func(t *testing.T) {
		t.Setenv("ENV", "production")
		t.Setenv("JWT_SECRET", "strong-prod-secret")
		t.Setenv("COOKIE_SECURE", "true")
		t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, *")

		_, err := LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wildcard '*'")

		assert.Panics(t, func() {
			_ = Load()
		})
	})

	t.Run("succeeds with valid production env", func(t *testing.T) {
		t.Setenv("ENV", "production")
		t.Setenv("JWT_SECRET", "strong-prod-secret")
		t.Setenv("COOKIE_SECURE", "true")
		t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, https://drive.example.com")

		cfg, err := LoadConfig()
		assert.NoError(t, err)
		assert.True(t, cfg.IsProduction)
		assert.True(t, cfg.CookieSecure)
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
