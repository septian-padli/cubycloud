package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/uptrace/bun"
)

const instanceIDSetting = "instance_id"

// InstanceID returns the installation ID, creating it once in the settings table.
func InstanceID(ctx context.Context, db *bun.DB) (string, error) {
	var id string
	if err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, instanceIDSetting).Scan(&id); err == nil {
		return id, nil
	} else if err != sql.ErrNoRows {
		return "", err
	}

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	generated := hex.EncodeToString(raw[:])
	if _, err := db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO NOTHING`, instanceIDSetting, generated); err != nil {
		return "", err
	}
	if err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, instanceIDSetting).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func BuildBlobKey(instanceID, nodeID, blobID string) string {
	return "cubycloud/" + instanceID + "/" + nodeID + "/blobs/" + blobID
}

func ParseBlobKey(key string) (instanceID, nodeID, blobID string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 5 || parts[0] != "cubycloud" || parts[1] == "" || parts[2] == "" || parts[3] != "blobs" || parts[4] == "" {
		return "", "", "", false
	}
	if parts[4] == "." || parts[4] == ".." || filepath.IsAbs(parts[4]) {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[4], true
}
