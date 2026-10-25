package storage

import (
	"context"
	"strconv"
	"strings"
)

// DetectDiskKind returns "SSD", "HDD", or "UNKNOWN" for the physical device backing path.
// It returns "UNKNOWN" when hardware detection is unavailable or inconclusive.
func DetectDiskKind(ctx context.Context, path string) string {
	if kind, err := platformDiskKind(ctx, path); err == nil && (kind == "SSD" || kind == "HDD") {
		return kind
	}
	return "UNKNOWN"
}

// Kernel naming has two partition styles:
//   - classic: "<letters><digits>" (sda1, vdb3, xvdf2) — trailing digits are the partition
//   - digitful devices (nvme0n1, mmcblk0, loop0): partition appends "p<digits>" (nvme0n1p2)
//
// Names without a partition suffix (sda, nvme0n1, mmcblk0) are returned as-is.
// Rare exotic sources (md/loop whole-dev) degrade to a sysfs miss and fall back
// to the path-name heuristic.
func blockDeviceName(dev string) (string, bool) {
	name := strings.TrimPrefix(dev, "/dev/")
	if name == "" {
		return "", false
	}
	// Style 2: "<base>p<digits>" with a digit right before the 'p'.
	if i := strings.LastIndexByte(name, 'p'); i > 0 && isDigits(name[i+1:]) && isDigitByte(name[i-1]) {
		return name[:i], true
	}
	// Style 1: classic "<letters><digits>".
	i := len(name)
	for i > 0 && isDigitByte(name[i-1]) {
		i--
	}
	if i > 0 && i < len(name) && isLowerLetters(name[:i]) {
		return name[:i], true
	}
	// No partition suffix: the name is the device itself.
	return name, true
}

func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range s {
		if !isDigitByte(s[i]) {
			return false
		}
	}
	return true
}

func isLowerLetters(s string) bool {
	if s == "" {
		return false
	}
	for i := range s {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// kindFromRotational parses /sys/block/<dev>/queue/rotational content.
func kindFromRotational(content string) (string, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(content), 10, 64)
	if err != nil {
		return "", false
	}
	if v == 1 {
		return "HDD", true
	}
	if v == 0 {
		return "SSD", true
	}
	return "", false
}

// deviceFromMounts finds the mount source for the longest mount point that
// prefixes path. mounts is the content of /proc/self/mounts.
func deviceFromMounts(mounts, path string) (string, bool) {
	bestDev := ""
	bestLen := -1
	for _, line := range strings.Split(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		mp := unescapeMountPath(fields[1])
		if mp == "/" || path == mp || strings.HasPrefix(path, mp+"/") {
			if len(mp) > bestLen {
				bestLen = len(mp)
				bestDev = fields[0]
			}
		}
	}
	if bestDev == "" {
		return "", false
	}
	return bestDev, true
}

// unescapeMountPath decodes octal escapes used in /proc/self/mounts (e.g. \040).
func unescapeMountPath(p string) string {
	if !strings.Contains(p, "\\") {
		return p
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' && i+3 < len(p) {
			if n, err := strconv.ParseInt(p[i+1:i+4], 8, 16); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(p[i])
	}
	return b.String()
}
