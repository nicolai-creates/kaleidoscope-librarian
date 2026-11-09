// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type mount struct{ dir, source string }

// mounts reads the mount table of this process.
func mounts() []mount {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil
	}
	var ms []mount
	for _, line := range strings.Split(string(data), "\n") {
		pre, post, ok := strings.Cut(line, " - ")
		f, g := strings.Fields(pre), strings.Fields(post)
		if !ok || len(f) < 5 || len(g) < 2 {
			continue
		}
		ms = append(ms, mount{unescapeMount(f[4]), g[1]})
	}
	return ms
}

// unescapeMount decodes the octal escapes (\040 for a space) in mountinfo.
func unescapeMount(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				sb.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// findVolume finds the mount point of the filesystem with the given label:
// by its device (/dev/disk/by-label), else by a mount point named like the
// label (/run/media/$USER/KLRU, /media/$USER/KLRU).
func findVolume(label string) (string, bool) {
	ms := mounts()
	if dev, err := filepath.EvalSymlinks("/dev/disk/by-label/" + label); err == nil {
		for _, m := range ms {
			if m.source == dev {
				return m.dir, true
			}
		}
	}
	for _, m := range ms {
		if filepath.Base(m.dir) == label {
			return m.dir, true
		}
	}
	return "", false
}
