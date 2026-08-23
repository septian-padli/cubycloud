package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DataDirPath string
	JWTSecret   string
	DBPath      string
	StoragePath string
	TempTUSPath string
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	dataDir := getEnv("DATA_DIR_PATH", "/app/data")
	jwtSecret := getEnv("JWT_SECRET", "")

	if jwtSecret == "" {
		panic("JWT_SECRET required")
	}

	return &Config{
		Port:        port,
		DataDirPath: dataDir,
		JWTSecret:   jwtSecret,
		DBPath:      dataDir + "/db/cubycloud.db",
		StoragePath: dataDir + "/storage/",
		TempTUSPath: dataDir + "/temp/tus/",
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
