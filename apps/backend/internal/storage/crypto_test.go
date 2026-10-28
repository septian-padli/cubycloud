package storage

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"github.com/cubycloud/backend/internal/database"
)

func TestMasterKeyInitializationAndReload(t *testing.T) {
	dataDir := t.TempDir()
	key, err := InitMasterKey(dataDir, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dataDir, "secrets", "master.key"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("master key file permissions: info=%v err=%v", info, err)
	}
	dirInfo, err := os.Stat(filepath.Join(dataDir, "secrets"))
	if err != nil || dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("master key directory permissions: info=%v err=%v", dirInfo, err)
	}
	reloaded, err := InitMasterKey(dataDir, "", "", nil)
	if err != nil || !bytes.Equal(key, reloaded) {
		t.Fatalf("reload key differs: err=%v", err)
	}
}

func TestMasterKeyOverridesAndValidation(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, masterKeyLen)
	encoded := base64.StdEncoding.EncodeToString(key)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets", "master.key"), bytes.Repeat([]byte{1}, masterKeyLen), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := InitMasterKey(dir, encoded, "", nil)
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("env override failed: %v", err)
	}
	if _, err := InitMasterKey(dir, "bad-key", "", nil); err == nil {
		t.Fatal("invalid env key accepted")
	}
	file := filepath.Join(dir, "provided.key")
	if err := os.WriteFile(file, []byte(encoded), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = InitMasterKey(dir, "", file, nil)
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("secret file key failed: %v", err)
	}
}

func TestMissingMasterKeyWithCredentialsFailsClosed(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO storage_nodes(id,type,access_key_encrypted,secret_key_encrypted,capacity_bytes,enabled) VALUES('cloud','S3',x'01',x'02',1,1)`); err != nil { t.Fatal(err) }
	if _, err := InitMasterKey(dataDir, "", "", db); err == nil { t.Fatal("missing key was silently regenerated") }
	if _, err := os.Stat(filepath.Join(dataDir, "secrets", "master.key")); !os.IsNotExist(err) { t.Fatalf("unexpected key file after failure: %v", err) }
}

func TestCredentialEnvelopeRoundtripNonceAndTamper(t *testing.T) {
	key := bytes.Repeat([]byte{9}, masterKeyLen)
	first, err := EncryptCredential([]byte("s3-secret"), key)
	if err != nil { t.Fatal(err) }
	second, err := EncryptCredential([]byte("s3-secret"), key)
	if err != nil { t.Fatal(err) }
	if bytes.Equal(first, second) { t.Fatal("nonces were reused") }
	plain, err := DecryptCredential(first, key)
	if err != nil || string(plain) != "s3-secret" { t.Fatalf("roundtrip: %q, %v", plain, err) }
	first[len(first)-1] ^= 1
	if _, err := DecryptCredential(first, key); err == nil { t.Fatal("tampered envelope accepted") }
}

func TestCredentialUsesInitializedKey(t *testing.T) {
	key := bytes.Repeat([]byte{7}, masterKeyLen)
	if _, err := InitMasterKey(t.TempDir(), base64.StdEncoding.EncodeToString(key), "", nil); err != nil { t.Fatal(err) }
	sealed, err := EncryptCredentialString("access-secret")
	if err != nil { t.Fatal(err) }
	plain, err := DecryptCredentialString(sealed)
	if err != nil || plain != "access-secret" { t.Fatalf("roundtrip=%q err=%v", plain, err) }
}

func TestRestoreWithSuppliedKeyInNewDataDirectory(t *testing.T) {
	key := bytes.Repeat([]byte{0x31}, masterKeyLen)
	if _, err := InitMasterKey(t.TempDir(), base64.StdEncoding.EncodeToString(key), "", nil); err != nil { t.Fatal(err) }
	envelope, err := EncryptCredentialString("provider-secret")
	if err != nil { t.Fatal(err) }
	if _, err := InitMasterKey(t.TempDir(), base64.StdEncoding.EncodeToString(key), "", nil); err != nil { t.Fatal(err) }
	got, err := DecryptCredentialString(envelope)
	if err != nil || got != "provider-secret" { t.Fatalf("restore decrypt=%q err=%v", got, err) }
}
