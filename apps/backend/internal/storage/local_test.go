package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalWriteReadDelete(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewLocalDriver(tmpDir)
	ctx := context.Background()

	content := []byte("test file content")
	filename := "test.txt"

	// Write
	if err := d.WriteLocal(ctx, "", filename, bytes.NewReader(content)); err != nil {
		t.Fatalf("WriteLocal failed: %v", err)
	}

	// Read
	r, err := d.ReadLocal(ctx, "", filename)
	if err != nil {
		t.Fatalf("ReadLocal failed: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, content) {
		t.Fatalf("got %s, want %s", got, content)
	}

	// Delete
	if err := d.DeleteLocal(ctx, "", filename); err != nil {
		t.Fatalf("DeleteLocal failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, filename)); !os.IsNotExist(err) {
		t.Fatal("file still exists after delete")
	}
}

func TestReadLocalNotExist(t *testing.T) {
	d := NewLocalDriver(t.TempDir())
	ctx := context.Background()
	_, err := d.ReadLocal(ctx, "", "ghost.txt")
	if err == nil {
		t.Fatal("expected error reading nonexistent file")
	}
}

func TestDeleteLocalNotExist(t *testing.T) {
	d := NewLocalDriver(t.TempDir())
	ctx := context.Background()
	err := d.DeleteLocal(ctx, "", "ghost.txt")
	if err == nil {
		t.Fatal("expected error deleting nonexistent file")
	}
}

func TestStatLocal(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewLocalDriver(tmpDir)
	ctx := context.Background()

	capacity, free, err := d.StatLocal(ctx, "")
	if err != nil {
		t.Fatalf("StatLocal failed: %v", err)
	}
	if capacity <= 0 {
		t.Fatalf("capacity %d should be > 0", capacity)
	}
	if free <= 0 {
		t.Fatalf("free %d should be > 0", free)
	}
	if free > capacity {
		t.Fatalf("free %d > capacity %d", free, capacity)
	}
}

func TestWriteLocalMkdirAll(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewLocalDriver(tmpDir)
	ctx := context.Background()

	nested := "a/b/c/file.txt"
	content := []byte("nested")
	if err := d.WriteLocal(ctx, "", nested, bytes.NewReader(content)); err != nil {
		t.Fatalf("WriteLocal nested failed: %v", err)
	}

	fullPath := filepath.Join(tmpDir, nested)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatal("nested file not created")
	}
}

func TestLocalNamespaceKey(t *testing.T) {
	root := t.TempDir()
	d := NewLocalDriver(root)
	ctx := context.Background()
	key := BuildBlobKey("instance", "node", "blob")
	content := []byte("namespaced")
	if err := d.WriteLocal(ctx, "", key, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	r, err := d.ReadLocal(ctx, "", key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("read namespaced blob = %q, %v", got, err)
	}
	if err := d.DeleteLocal(ctx, "", key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(key))); !os.IsNotExist(err) {
		t.Fatalf("namespaced blob remains: %v", err)
	}
}
