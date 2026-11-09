// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// findVolume finds the volume with the given label under /Volumes.
func findVolume(label string) (string, bool) {
	dir := filepath.Join("/Volumes", label)
	st, err := os.Stat(dir)
	return dir, err == nil && st.IsDir()
}

// ejectVolume ejects the volume with diskutil.
func ejectVolume(dir string) error {
	out, err := exec.Command("diskutil", "eject", dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("diskutil eject %s: %s", dir, strings.TrimSpace(string(out)))
	}
	return nil
}
