package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/uptrace/bun"
)

const masterKeyLen = 32

var masterKeyState struct {
	sync.RWMutex
	key []byte
	err error
}

// InitMasterKey resolves and caches the credential key for this process.
func InitMasterKey(dataDir, envKey, secretFilePath string, db *bun.DB) ([]byte, error) {
	var key []byte
	var err error
	switch {
	case envKey != "":
		key, err = base64.StdEncoding.Strict().DecodeString(envKey)
		if err == nil && len(key) != masterKeyLen {
			err = errors.New("master key must be a base64-encoded 32-byte value")
		}
	case secretFilePath != "":
		var raw []byte
		raw, err = os.ReadFile(secretFilePath)
		if err == nil {
			key, err = parseMasterKeyFile(raw)
		}
	default:
		key, err = loadOrCreateMasterKey(filepath.Join(dataDir, "secrets", "master.key"), db)
	}
	if err != nil {
		setMasterKey(nil, err)
		return nil, err
	}
	setMasterKey(key, nil)
	return append([]byte(nil), key...), nil
}

// GetMasterKey returns a copy of the initialized credential key.
func GetMasterKey() ([]byte, error) {
	masterKeyState.RLock()
	defer masterKeyState.RUnlock()
	if masterKeyState.err != nil {
		return nil, masterKeyState.err
	}
	if len(masterKeyState.key) != masterKeyLen {
		return nil, errors.New("master key has not been initialized")
	}
	return append([]byte(nil), masterKeyState.key...), nil
}

func setMasterKey(key []byte, err error) {
	masterKeyState.Lock()
	masterKeyState.key = append([]byte(nil), key...)
	masterKeyState.err = err
	masterKeyState.Unlock()
}

func parseMasterKeyFile(raw []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(raw))
	if len(raw) == masterKeyLen {
		return append([]byte(nil), raw...), nil
	}
	key, err := base64.StdEncoding.Strict().DecodeString(trimmed)
	if err != nil || len(key) != masterKeyLen {
		return nil, errors.New("master key file must contain a 32-byte key or base64-encoded 32-byte key")
	}
	return key, nil
}

func loadOrCreateMasterKey(path string, db *bun.DB) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		return parseMasterKeyFile(key)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master key: %w", err)
	}
	if db != nil {
		var hasCredentials int
		err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM storage_nodes WHERE access_key_encrypted IS NOT NULL AND access_key_encrypted != '')`).Scan(&hasCredentials)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("check encrypted cloud credentials: %w", err)
		}
		if hasCredentials != 0 {
			return nil, errors.New("master key is missing while encrypted cloud credentials exist")
		}
	}
	key = make([]byte, masterKeyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, errors.New("generate master key")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create master key directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("secure master key directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".master-key-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary master key: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("secure temporary master key: %w", err)
	}
	_, writeErr := tmp.Write(key)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return nil, fmt.Errorf("persist master key: %w", err)
	}
	if err := os.Link(tmpPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			stored, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil, fmt.Errorf("read concurrently created master key: %w", readErr)
			}
			return parseMasterKeyFile(stored)
		}
		return nil, fmt.Errorf("persist master key: %w", err)
	}
	return key, nil
}

// ResolveKeyMaterial is retained for existing byte-oriented storage call sites.
func ResolveKeyMaterial() []byte {
	key, _ := GetMasterKey()
	return key
}

// EncryptCredential encrypts plaintext with a versioned AES-256-GCM envelope.
// An optional key argument supports internal callers that already hold a key.
func EncryptCredential(plaintext []byte, keyMaterial ...[]byte) ([]byte, error) {
	key, err := credentialKey(keyMaterial)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("initialize credential encryption")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("initialize credential encryption")
	}
	envelope := make([]byte, 1+gcm.NonceSize(), 1+gcm.NonceSize()+len(plaintext)+gcm.Overhead())
	envelope[0] = 1
	if _, err := io.ReadFull(rand.Reader, envelope[1:]); err != nil {
		return nil, errors.New("generate credential nonce")
	}
	return gcm.Seal(envelope, envelope[1:], plaintext, []byte{1}), nil
}

// DecryptCredential authenticates and decrypts a versioned AES-256-GCM envelope.
func DecryptCredential(envelope []byte, keyMaterial ...[]byte) ([]byte, error) {
	key, err := credentialKey(keyMaterial)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("initialize credential decryption")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("initialize credential decryption")
	}
	if len(envelope) < 1+gcm.NonceSize()+gcm.Overhead() || envelope[0] != 1 {
		return nil, errors.New("invalid credential envelope")
	}
	plaintext, err := gcm.Open(nil, envelope[1:1+gcm.NonceSize()], envelope[1+gcm.NonceSize():], []byte{1})
	if err != nil {
		return nil, errors.New("credential authentication failed")
	}
	return plaintext, nil
}

func credentialKey(explicit [][]byte) ([]byte, error) {
	if len(explicit) != 0 {
		if len(explicit[0]) != masterKeyLen {
			return nil, errors.New("master key must be 32 bytes")
		}
		return explicit[0], nil
	}
	return GetMasterKey()
}

// EncryptCredentialString encrypts UTF-8 credentials using the initialized key.
func EncryptCredentialString(plaintext string) (string, error) {
	sealed, err := EncryptCredential([]byte(plaintext))
	return base64.RawStdEncoding.EncodeToString(sealed), err
}

// DecryptCredentialString decrypts a base64 envelope using the initialized key.
func DecryptCredentialString(ciphertext string) (string, error) {
	sealed, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errors.New("invalid credential envelope")
	}
	plain, err := DecryptCredential(sealed)
	return string(plain), err
}
