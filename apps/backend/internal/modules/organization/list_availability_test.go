package organization_test

import (
	"context"
	"testing"

	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/storage"
)

type listAvailabilityDriver struct {
	storage.StorageDriver
	unavailable map[string]bool
}

func (d *listAvailabilityDriver) ProbeNode(_ context.Context, nodeID string) error {
	if d.unavailable[nodeID] {
		return storage.ErrUnavailable
	}
	return nil
}

func TestListPreservesFilesAndReportsStorageAvailability(t *testing.T) {
	env := setupTestEnv(t)
	defer env.db.Close()
	ctx := context.Background()
	insertTestUser(t, env.db, "list-user", "listuser", "REGULAR", 100000)

	for _, node := range []struct {
		id   string
		path string
	}{{"online-org-node", "/online"}, {"offline-org-node", "/offline"}} {
		if _, err := env.db.ExecContext(ctx, `INSERT INTO storage_nodes(id,name,type,path,capacity_bytes,enabled) VALUES(?, ?, 'LOCAL', ?, 1024, 1)`, node.id, node.id, node.path); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		id, name, node string
	}{{"online-org-file", "online.txt", "online-org-node"}, {"offline-org-file", "offline.txt", "offline-org-node"}} {
		if _, err := env.db.ExecContext(ctx, `INSERT INTO organization_items(id,created_by,name,kind,size_bytes,disk_key,storage_node_id) VALUES(?,?,?,'file',12,?,?)`, item.id, "list-user", item.name, item.id+"-blob", item.node); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.db.ExecContext(ctx, `INSERT INTO organization_items(id,created_by,name,kind,size_bytes,storage_node_id) VALUES('offline-org-folder','list-user','folder','folder',0,'offline-org-node')`); err != nil {
		t.Fatal(err)
	}
	driver := &listAvailabilityDriver{StorageDriver: env.driver, unavailable: map[string]bool{"offline-org-node": true}}
	service := organization.NewService(env.db, driver)
	items, _, err := service.List(ctx, "list-user", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want both files and the folder retained", len(items))
	}
	got := make(map[string]organization.Item, len(items))
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
