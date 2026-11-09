// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// mountOf returns the mount that holds dir (the longest matching mount point).
func mountOf(dir string) (mount, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return mount{}, false
	}
	var best mount
	for _, m := range mounts() {
		if (abs == m.dir || strings.HasPrefix(abs, strings.TrimSuffix(m.dir, "/")+"/")) && len(m.dir) > len(best.dir) {
			best = m
		}
	}
	return best, best.dir != ""
}

// ejectVolume unmounts the drive with udisksctl and powers it off. Without
// udisks it only flushes the data.
func ejectVolume(dir string) error {
	syscall.Sync()
	m, ok := mountOf(dir)
	if !ok || !strings.HasPrefix(m.source, "/dev/") {
		return fmt.Errorf("%s: no mounted drive found", dir)
	}
	if _, err := exec.LookPath("udisksctl"); err != nil {
		return fmt.Errorf("data flushed, but udisksctl is not installed: unmount %s in your file manager", m.dir)
	}
	run := func(verb string) error {
		out, err := exec.Command("udisksctl", verb, "--no-user-interaction", "-b", m.source).CombinedOutput()
		if err != nil {
			return fmt.Errorf("udisksctl %s %s: %s", verb, m.source, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := run("unmount"); err != nil {
		return err
	}
	if err := run("power-off"); err != nil {
		return ejectWarning{fmt.Errorf("power-off failed: %v", err)}
	}
	return nil
}
