package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigLoad_Defaults(t *testing.T) {
	// Clear any env
	os.Unsetenv("PORT")
	os.Unsetenv("DATA_DIR_PATH")
	os.Unsetenv("JWT_SECRET")

	cfg := Load()
	assert.Equal(t, "8383", cfg.Port)
	assert.NotEmpty(t, cfg.DataDirPath)
	assert.NotEmpty(t, cfg.JWTSecret)
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
