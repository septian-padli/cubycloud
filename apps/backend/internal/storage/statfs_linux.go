//go:build linux

package storage

import (
	"fmt"
	"syscall"
)

func statfsCapacity(path string) (capacity, free int64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, fmt.Errorf("statfs failed: %w", err)
	}

	// On Linux (especially with virtiofs / FUSE / NFS mounts), stat.Bsize is the optimal
	// transfer block size (e.g. 1MB), while stat.Frsize is the fundamental allocation unit
	// (e.g. 4KB). stat.Blocks and stat.Bavail are counted in units of stat.Frsize.
	blockSize := int64(stat.Bsize)
	if stat.Frsize > 0 {
		blockSize = int64(stat.Frsize)
	}

	capacity = int64(stat.Blocks) * blockSize
	free = int64(stat.Bavail) * blockSize
	return capacity, free, nil
}
