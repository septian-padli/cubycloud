//go:build darwin

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

	capacity = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	return capacity, free, nil
}
