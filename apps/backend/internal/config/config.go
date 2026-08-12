package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultPort    = "8080"
	defaultDataDir = "./data"
)

// Config is boot-time process config from OS env only (ARCHITECTURE §3.1).
// App-level settings live in SQLite later — not here.
type Config struct {
	Port        string
	DataDirPath string
}

func Load() Config {
	return Config{
		Port:        getenv("PORT", defaultPort),
		DataDirPath: getenv("DATA_DIR_PATH", defaultDataDir),
	}
}

func (c Config) Addr() string {
	return ":" + c.Port
}

func (c Config) DBDir() string {
	return filepath.Join(c.DataDirPath, "db")
}

func (c Config) StorageDir() string {
	return filepath.Join(c.DataDirPath, "storage")
}

func (c Config) TempDir() string {
	return filepath.Join(c.DataDirPath, "temp")
}

func (c Config) EnsureDataDirs() error {
	for _, dir := range []string{c.DBDir(), c.StorageDir(), c.TempDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create data dir %s: %w", dir, err)
		}
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
