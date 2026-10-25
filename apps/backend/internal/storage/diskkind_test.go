package storage

import (
	"context"
	"testing"
)

func TestDetectDiskKind_UnknownFallback(t *testing.T) {
	got := DetectDiskKind(context.Background(), "/nonexistent/test/path")
	if got != "UNKNOWN" && got != "SSD" && got != "HDD" {
		t.Errorf("DetectDiskKind() = %q, want valid kind", got)
	}
}

func TestBlockDeviceName(t *testing.T) {
	cases := []struct {
		dev  string
		want string
		ok   bool
	}{
		{"/dev/sda1", "sda", true},
		{"/dev/sda", "sda", true},
		{"/dev/nvme0n1p2", "nvme0n1", true},
		{"/dev/nvme0n1", "nvme0n1", true},
		{"/dev/mmcblk0p1", "mmcblk0", true},
		{"/dev/vdb3", "vdb", true},
		{"tmpfs", "tmpfs", true}, // non-block source: returned as-is
		{"/dev/", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := blockDeviceName(c.dev)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("blockDeviceName(%q) = (%q, %v), want (%q, %v)", c.dev, got, ok, c.want, c.ok)
		}
	}
}

func TestKindFromRotational(t *testing.T) {
	if kind, ok := kindFromRotational("1\n"); !ok || kind != "HDD" {
		t.Errorf("rotational 1: got (%q, %v)", kind, ok)
	}
	if kind, ok := kindFromRotational("0\n"); !ok || kind != "SSD" {
		t.Errorf("rotational 0: got (%q, %v)", kind, ok)
	}
	if _, ok := kindFromRotational("yes\n"); ok {
		t.Error("rotational 'yes' should not parse")
	}
}

func TestDeviceFromMounts(t *testing.T) {
	mounts := "/dev/sda1 / ext4 rw 0 0\n" +
		"/dev/sdb1 /app/data ext4 rw 0 0\n" +
		"tmpfs /tmp tmpfs rw 0 0\n" +
		"/dev/nvme0n1p2 /mnt/my\\040data xfs rw 0 0\n"

	cases := []struct {
		path string
		want string
	}{
		{"/app/data/storage/default", "/dev/sdb1"},  // longest prefix wins
		{"/app/data", "/dev/sdb1"},                  // exact mount point
		{"/home/user/file", "/dev/sda1"},            // falls back to root
		{"/mnt/my data/node", "/dev/nvme0n1p2"},     // octal escape decoded
	}
	for _, c := range cases {
		got, ok := deviceFromMounts(mounts, c.path)
		if !ok || got != c.want {
			t.Errorf("deviceFromMounts(%q) = (%q, %v), want %q", c.path, got, ok, c.want)
		}
	}
}
