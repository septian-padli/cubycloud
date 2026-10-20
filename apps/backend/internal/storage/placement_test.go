package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cubycloud/backend/internal/database"
)

func TestAvailableOnNodeDeduplicatesAcrossStorageTables(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('node','S3',10000,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users(id,username,email,password_hash,role) VALUES('user','user','user@example.test','hash','REGULAR')`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		table, id, name, key string
		size                 int64
	}{
		{"my_storage_items", "my-shared", "my-shared.bin", "shared", 300},
		{"my_storage_items", "my-own", "my-own.bin", "my-own", 200},
		{"organization_items", "org-shared", "org-shared.bin", "shared", 350},
		{"organization_items", "org-own", "org-own.bin", "org-own", 400},
	} {
		query := `INSERT INTO ` + item.table + `(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id) VALUES(?, 'user', ?, 'file', ?, ?, 'node')`
		if item.table == "organization_items" {
			query = `INSERT INTO ` + item.table + `(id,created_by,name,kind,size_bytes,disk_key,storage_node_id) VALUES(?, 'user', ?, 'file', ?, ?, 'node')`
		}
		if _, err := db.ExecContext(ctx, query, item.id, item.name, item.size, item.key); err != nil {
			t.Fatal(err)
		}
	}

	available, err := AvailableOnNode(ctx, db, nil, "node")
	if err != nil {
		t.Fatal(err)
	}
	const want int64 = 8550 // 5% reserve + max(shared) + both distinct objects.
	if available != want {
		t.Fatalf("available bytes = %d, want %d", available, want)
	}

	used, err := NodeUsedBytes(ctx, db.DB, "node")
	if err != nil {
		t.Fatal(err)
	}
	if used != 950 {
		t.Fatalf("used bytes = %d, want 950", used)
	}

	// Test in-flight reservation impact on AvailableOnNode
	res := ReserveNode("node", 500)
	if reserved := GetNodeReserved("node"); reserved != 500 {
		t.Fatalf("reserved = %d, want 500", reserved)
	}
	availWithRes, err := AvailableOnNode(ctx, db, nil, "node")
	if err != nil {
		t.Fatal(err)
	}
	if availWithRes != 8050 {
		t.Fatalf("available with reservation = %d, want 8050", availWithRes)
	}
	res.Release()
	if reserved := GetNodeReserved("node"); reserved != 0 {
		t.Fatalf("reserved after release = %d, want 0", reserved)
	}
	availAfterRelease, err := AvailableOnNode(ctx, db, nil, "node")
	if err != nil {
		t.Fatal(err)
	}
	if availAfterRelease != 8550 {
		t.Fatalf("available after release = %d, want 8550", availAfterRelease)
	}

	// Test CheckNodeCapacityTx on local node
	if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES('local-node','LOCAL',1000,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id) VALUES('f1','user','f1.bin','file',500,'k1','local-node')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO organization_items(id,created_by,name,kind,size_bytes,disk_key,storage_node_id) VALUES('f2','user','f2.bin','file',400,'k2','local-node')`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	availableInTx, err := AvailableOnNodeTx(ctx, tx, nil, "node")
	if err != nil {
		t.Fatal(err)
	}
	if availableInTx != 8550 {
		t.Fatalf("available in transaction = %d, want 8550", availableInTx)
	}

	// Remaining capacity: 1000 - 50 (5% reserve) - 900 (used) = 50 bytes.
	if err := CheckNodeCapacityTx(ctx, tx, nil, "local-node", 40); err != nil {
		t.Fatalf("expected CheckNodeCapacityTx(40) to succeed, got %v", err)
	}
	if err := CheckNodeCapacityTx(ctx, tx, nil, "local-node", 60); err == nil {
		t.Fatalf("expected CheckNodeCapacityTx(60) to fail due to pool full")
	}
}
