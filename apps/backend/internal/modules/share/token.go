package share

import (
	"crypto/rand"
)

// URL-safe characters: 64 characters (6 bits per char) ensures uniform random distribution without modulo bias.
const tokenAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

// GenerateToken generates a URL-safe random token of length n (default 12, min 10, max 32) using crypto/rand.
func GenerateToken(length ...int) (string, error) {
	n := 12
	if len(length) > 0 && length[0] >= 10 && length[0] <= 32 {
		n = length[0]
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = tokenAlphabet[b[i]&63]
	}
	return string(b), nil
}
