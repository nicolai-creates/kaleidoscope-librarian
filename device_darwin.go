// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
)

// findVolume finds the volume with the given label under /Volumes.
func findVolume(label string) (string, bool) {
	dir := filepath.Join("/Volumes", label)
	st, err := os.Stat(dir)
	return dir, err == nil && st.IsDir()
}
