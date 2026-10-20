package storage
import (
	"bytes"
	"path/filepath"
	"testing"
)
func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := []byte("test-key-material")
	plaintext := []byte("s3-secret-key-value")

	ciphertext, err := EncryptCredential(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	decrypted, err := DecryptCredential(ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("got %s, want %s", decrypted, plaintext)
	}
}
func TestNodeDriverNormalizesDataDirPathForCredentials(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR_PATH", dataDir+"/")

	driver := newNodeDriver(nil, filepath.Join(dataDir, "storage")).(*nodeDriver)
	wantKey := []byte(filepath.Clean(dataDir))
	if !bytes.Equal(driver.keyMaterial, wantKey) {
		t.Fatalf("decryption key material = %q, want normalized path %q", driver.keyMaterial, wantKey)
	}

	ciphertext, err := EncryptCredential([]byte("secret"), wantKey)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	plaintext, err := DecryptCredential(ciphertext, driver.keyMaterial)
	if err != nil {
		t.Fatalf("decrypt credential with DATA_DIR_PATH trailing slash: %v", err)
	}
	if !bytes.Equal(plaintext, []byte("secret")) {
		t.Fatalf("decrypted credential = %q, want secret", plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := []byte("key-1")
	key2 := []byte("key-2")
	plaintext := []byte("secret")

	ciphertext, _ := EncryptCredential(plaintext, key1)
	_, err := DecryptCredential(ciphertext, key2)
	if err == nil {
		t.Fatal("expected decrypt to fail with wrong key")
	}
}

func TestDecryptShortCiphertext(t *testing.T) {
	key := []byte("key")
	_, err := DecryptCredential([]byte("short"), key)
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}
