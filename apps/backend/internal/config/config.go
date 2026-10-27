package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port                string
	DataDirPath         string
	JWTSecret           string
	DBPath              string
	StoragePath         string
	TempTUSPath         string
	IsProduction        bool
	CookieSecure        bool
	TrustProxy          bool
	AllowedOrigins      []string
	MasterKey           string
	MasterKeyFile       string
	StorageCleanupDryRun bool
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
		resolvedSecret, err := loadOrCreateJWTSecret(dataDir)
		if err != nil {
			return nil, err
		}
		jwtSecret = resolvedSecret
	}

	cookieSecureRaw := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	cookieSecure := strings.EqualFold(cookieSecureRaw, "true")
	trustProxy := strings.EqualFold(strings.TrimSpace(os.Getenv("TRUST_PROXY")), "true")

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
		for _, origin := range allowedOrigins {
			if origin == "*" {
				return nil, errors.New("ALLOWED_ORIGINS cannot contain wildcard '*' in production environment")
			}
		}
	} else {
		if len(originsRaw) == 0 {
			allowedOrigins = []string{"*"}
		}
	}
	masterKey := os.Getenv("CUBYCLOUD_MASTER_KEY")
	masterKeyFile := os.Getenv("CUBYCLOUD_MASTER_KEY_FILE")
	cleanupDryRun := true
	if val := strings.TrimSpace(os.Getenv("CUBYCLOUD_STORAGE_CLEANUP_DRY_RUN")); val != "" {
		cleanupDryRun = !strings.EqualFold(val, "false")
	} else if val := strings.TrimSpace(os.Getenv("CUBYCLOUD_STORAGE_CLEANUP_ENABLED")); strings.EqualFold(val, "true") {
		cleanupDryRun = false
	}

	return &Config{
		Port:                port,
		DataDirPath:         dataDir,
		JWTSecret:           jwtSecret,
		DBPath:              filepath.Join(dataDir, "db", "cubycloud.db"),
		StoragePath:         filepath.Join(dataDir, "storage"),
		TempTUSPath:         filepath.Join(dataDir, "temp", "tus"),
		IsProduction:        isProduction,
		CookieSecure:        cookieSecure,
		TrustProxy:          trustProxy,
		AllowedOrigins:      allowedOrigins,
		MasterKey:           masterKey,
		MasterKeyFile:       masterKeyFile,
		StorageCleanupDryRun: cleanupDryRun,
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

func loadOrCreateJWTSecret(dataDir string) (string, error) {
	secretPath := filepath.Join(dataDir, "secrets", "jwt.key")
	raw, err := os.ReadFile(secretPath)
	if err == nil {
		secret := strings.TrimSpace(string(raw))
		if len(secret) < 32 {
			return "", errors.New("persisted JWT secret in secrets/jwt.key is invalid (must be at least 32 characters)")
		}
		return secret, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read JWT secret file: %w", err)
	}

	// File does not exist. Check if an existing database exists.
	dbPath := filepath.Join(dataDir, "db", "cubycloud.db")
	if fi, statErr := os.Stat(dbPath); statErr == nil && fi.Size() > 0 {
		return "", errors.New("JWT secret is missing while existing database is present; restore secrets/jwt.key or provide JWT_SECRET environment variable")
	}

	// Clean install: generate random 32-byte secret (hex encoded = 64 characters)
	buf := make([]byte, 32)
	if _, randErr := io.ReadFull(rand.Reader, buf); randErr != nil {
		return "", fmt.Errorf("generate random JWT secret: %w", randErr)
	}
	newSecret := hex.EncodeToString(buf)

	secretsDir := filepath.Dir(secretPath)
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return "", fmt.Errorf("create secrets directory: %w", err)
	}
	_ = os.Chmod(secretsDir, 0700)

	tmp, err := os.CreateTemp(secretsDir, ".jwt-secret-*")
	if err != nil {
		return "", fmt.Errorf("create temporary JWT secret file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	_ = tmp.Chmod(0600)
	if _, err := tmp.WriteString(newSecret + "\n"); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temporary JWT secret: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("sync temporary JWT secret: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary JWT secret: %w", err)
	}

	if err := os.Link(tmpName, secretPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, rErr := os.ReadFile(secretPath)
			if rErr == nil && len(strings.TrimSpace(string(existing))) >= 32 {
				return strings.TrimSpace(string(existing)), nil
			}
		}
		return "", fmt.Errorf("persist JWT secret: %w", err)
	}

	log.Printf("[INFO] Auto-generated and persisted new JWT secret to %s", secretPath)
	return newSecret, nil
}
