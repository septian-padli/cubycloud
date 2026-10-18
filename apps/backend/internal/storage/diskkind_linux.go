//go:build linux

package storage

import (
	"context"
	"fmt"
	"os"
)

// platformDiskKind detects SSD/HDD via /sys/block/<dev>/queue/rotational.
func platformDiskKind(ctx context.Context, path string) (string, error) {
	_ = ctx
	mounts, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return "", err
	}
	dev, ok := deviceFromMounts(string(mounts), path)
	if !ok {
		return "", fmt.Errorf("no mount entry for %s", path)
	}
	base, ok := blockDeviceName(dev)
	if !ok {
		return "", fmt.Errorf("cannot derive block device from %s", dev)
	}
	content, err := os.ReadFile("/sys/block/"+base+"/queue/rotational")
	if err != nil {
		return "", err
	}
	kind, ok := kindFromRotational(string(content))
	if !ok {
		return "", fmt.Errorf("invalid rotational value for %s", base)
	}
	return kind, nil
}
