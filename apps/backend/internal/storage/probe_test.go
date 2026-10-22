package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cubycloud/backend/internal/database"
)

type probeTestDriver struct {
	StorageDriver
	unavailable map[string]bool
	probed      []string
}

func (d *probeTestDriver) ProbeNode(_ context.Context, nodeID string) error {
	d.probed = append(d.probed, nodeID)
	if d.unavailable[nodeID] {
		return ErrUnavailable
	}
	return nil
}

func TestAllConfiguredNodesAvailableProbesEveryNode(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, id := range []string{"local-node", "s3-node", "disabled-node"} {
		_, err := db.ExecContext(context.Background(), `INSERT INTO storage_nodes(id,type,capacity_bytes,enabled) VALUES(?,?,?,?)`, id, "LOCAL", 1, id != "disabled-node")
		if err != nil {
			t.Fatal(err)
		}
	}

	driver := &probeTestDriver{unavailable: map[string]bool{}}
	if !AllConfiguredNodesAvailable(context.Background(), db, driver) {
		t.Fatal("a reachable disabled node must not make health fail")
	}
	if len(driver.probed) != 3 {
		t.Fatalf("probed %d registered nodes, want all three; probed %v", len(driver.probed), driver.probed)
	}

	driver.unavailable = map[string]bool{"disabled-node": true}
	driver.probed = nil
	if AllConfiguredNodesAvailable(context.Background(), db, driver) {
		t.Fatal("an unavailable disabled node must make health fail")
	}
	if len(driver.probed) != 3 {
		t.Fatalf("probed %d registered nodes, want all three; probed %v", len(driver.probed), driver.probed)
	}

	driver.unavailable = map[string]bool{"s3-node": true}
	driver.probed = nil
	if AllConfiguredNodesAvailable(context.Background(), db, driver) {
		t.Fatal("an unavailable enabled node must make health fail")
	}
	if len(driver.probed) != 3 {
		t.Fatalf("probed %d registered nodes, want all three; probed %v", len(driver.probed), driver.probed)
	}
}

