package database

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/oklog/ulid/v2"
)

func GenerateAuthToken() (string, string, string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", "", err
	}
	hash := sha256.Sum256(secret)
	return hex.EncodeToString(secret), hex.EncodeToString(hash[:]), ulid.Make().String(), nil
}
