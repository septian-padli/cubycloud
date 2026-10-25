//go:build !linux && !darwin

package storage

import (
	"fmt"
	"runtime"
)

func statfsCapacity(path string) (capacity, free int64, err error) {
	// ponytail: Windows GetDiskFreeSpaceEx — implement when Windows container needed.
	return 0, 0, fmt.Errorf("%s statfs not implemented", runtime.GOOS)
}
