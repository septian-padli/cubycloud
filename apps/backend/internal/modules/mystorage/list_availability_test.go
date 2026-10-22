package mystorage

import (
	"context"
	"testing"

	"github.com/cubycloud/backend/internal/storage"
)

type listAvailabilityDriver struct {
	*mockStorageDriver
	unavailable map[string]bool
}

func (d *listAvailabilityDriver) ProbeNode(_ context.Context, nodeID string) error {
	if d.unavailable[nodeID] {
		return storage.ErrUnavailable
	}
	return nil
}

func TestListPreservesFilesAndReportsStorageAvailability(t *testing.T) {
	db, _, baseDriver := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	for _, node := range []struct {
		id   string
		path string
	}{{"online-node", "/online"}, {"offline-node", "/offline"}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES(?, ?, 'LOCAL', ?, 1024, 1)`, node.id, node.id, node.path); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		id, name, node string
	}{{"online-file", "online.txt", "online-node"}, {"offline-file", "offline.txt", "offline-node"}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,disk_key,storage_node_id) VALUES(?, 'user-1', ?, 'file', 12, ?, ?)`, item.id, item.name, item.id+"-blob", item.node); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO my_storage_items(id,owner_id,name,kind,size_bytes,storage_node_id) VALUES('offline-associated-folder','user-1','folder','folder',0,'offline-node')`); err != nil {
		t.Fatal(err)
	}
	driver := &listAvailabilityDriver{mockStorageDriver: baseDriver, unavailable: map[string]bool{"offline-node": true}}
	service := NewService(db, driver)
	items, _, err := service.List(ctx, "user-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want both files and the folder retained", len(items))
	}
	got := make(map[string]Item, len(items))
	for _, item := range items {
		got[item.Name] = item
	}
	if item, ok := got["online.txt"]; !ok || !item.StorageAvailable || item.SizeBytes != 12 {
		t.Errorf("online file metadata/availability not preserved: %+v, present=%t", item, ok)
	}
	if item, ok := got["offline.txt"]; !ok || item.StorageAvailable || item.SizeBytes != 12 {
		t.Errorf("offline file should remain listed with metadata: %+v, present=%t", item, ok)
	}
	if item, ok := got["folder"]; !ok || !item.StorageAvailable {
		t.Errorf("folder should remain available despite its associated node: %+v, present=%t", item, ok)
	}
}
