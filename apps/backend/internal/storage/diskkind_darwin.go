//go:build darwin

package storage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// platformDiskKind detects SSD/HDD via `diskutil info <device>` on macOS.
// path may not exist yet (e.g. the fabricated default node); detection then
// walks up to the nearest existing ancestor, which lives on the same volume.
func platformDiskKind(ctx context.Context, path string) (string, error) {
	var st syscall.Statfs_t
	p := path
	for {
		if err := syscall.Statfs(p, &st); err == nil {
			break
		} else if !errors.Is(err, syscall.ENOENT) || p == filepath.Dir(p) {
			return "", err
		}
		p = filepath.Dir(p)
	}
	dev := make([]byte, 0, len(st.Mntfromname))
	for _, c := range st.Mntfromname {
		if c == 0 {
			break
		}
		dev = append(dev, byte(c))
	}
	if len(dev) == 0 {
		return "", fmt.Errorf("empty mount source for %s", path)
	}

	out, err := exec.CommandContext(ctx, "diskutil", "info", string(dev)).Output()
	if err != nil {
		return "", err
	}
	// diskutil aligns columns, so collapse whitespace before matching.
	flat := strings.Join(strings.Fields(string(out)), " ")
	switch {
	case strings.Contains(flat, "Solid State: Yes"):
		return "SSD", nil
	case strings.Contains(flat, "Solid State: No"):
		return "HDD", nil
	}
	return "", fmt.Errorf("cannot determine disk kind for %s", string(dev))
}

