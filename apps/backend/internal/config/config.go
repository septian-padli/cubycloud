package config

import (
	"bufio"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	DataDirPath    string
	JWTSecret      string
	DBPath         string
	StoragePath    string
	TempTUSPath    string
	IsProduction   bool
	CookieSecure   bool
	AllowedOrigins []string
}

func Load() *Config {
	loadDotEnv()
	cfg, err := LoadConfig()
	if err != nil {
		panic(err.Error())
	}
	return cfg
}

// LoadConfig parses and validates configuration from environment variables.
func LoadConfig() (*Config, error) {
	port := getEnv("PORT", "8383")

	// Determine DataDirPath with sensible defaults:
	// 1. DATA_DIR_PATH env if specified
	// 2. /app/data if accessible (e.g. running in Docker)
	// 3. ./data for host/local development
	dataDir := os.Getenv("DATA_DIR_PATH")
	if dataDir == "" {
		if isDirWritable("/app/data") {
			dataDir = "/app/data"
		} else {
			dataDir = "./data"
		}
	}
	dataDir = filepath.Clean(dataDir)

	env := strings.ToLower(os.Getenv("ENV"))
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	isProduction := env == "production" || appEnv == "production"

	// Determine JWTSecret:
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if isProduction {
			return nil, errors.New("JWT_SECRET required in production environment")
		}
		jwtSecret = "cubycloud-dev-secret-change-in-production"
		log.Println("[WARN] JWT_SECRET not set. Using default development secret. DO NOT use in production!")
	}

	cookieSecureRaw := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	if isProduction {
		if !strings.EqualFold(cookieSecureRaw, "true") {
			return nil, errors.New("COOKIE_SECURE=true required in production environment")
		}
	}
	cookieSecure := strings.EqualFold(cookieSecureRaw, "true")

	originsRaw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	var allowedOrigins []string
	if originsRaw != "" {
		for _, p := range strings.Split(originsRaw, ",") {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	if isProduction {
		if len(allowedOrigins) == 0 {
			return nil, errors.New("ALLOWED_ORIGINS required in production environment")
		}
		for _, origin := range allowedOrigins {
			if origin == "*" {
				return nil, errors.New("ALLOWED_ORIGINS cannot contain wildcard '*' in production environment")
			}
		}
	} else {
		if len(allowedOrigins) == 0 {
			allowedOrigins = []string{"*"}
		}
	}

	return &Config{
		Port:           port,
		DataDirPath:    dataDir,
		JWTSecret:      jwtSecret,
		DBPath:         filepath.Join(dataDir, "db", "cubycloud.db"),
		StoragePath:    filepath.Join(dataDir, "storage"),
		TempTUSPath:    filepath.Join(dataDir, "temp", "tus"),
		IsProduction:   isProduction,
		CookieSecure:   cookieSecure,
		AllowedOrigins: allowedOrigins,
	}, nil
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

// loadDotEnv tries to load environment variables from .env files without external dependencies.
func loadDotEnv() {
	candidates := []string{
		".env",
		"apps/backend/.env",
		"../.env",
		"../../.env",
	}

	for _, path := range candidates {
		if parseDotEnvFile(path) {
			break
		}
	}
}

func parseDotEnvFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip surrounding quotes if present
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}

		// Only set if not already present in environment
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return true
}

func isDirWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		// If doesn't exist, try creating to test writability
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0755); err == nil {
				return true
			}
		}
		return false
	}
	if !info.IsDir() {
		return false
	}

	// Test write permission by creating and removing a test file
	testFile := filepath.Join(path, ".cubycloud_write_test")
	if err := os.WriteFile(testFile, []byte(""), 0600); err != nil {
		return false
	}
	_ = os.Remove(testFile)
	return true
}
