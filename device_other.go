// SPDX-License-Identifier: MIT

//go:build !linux && !darwin && !windows

package main

import "fmt"

// findVolume is not implemented on this platform; use KPM_DEVICE.
func findVolume(label string) (string, bool) { return "", false }

// ejectVolume is not implemented on this platform.
func ejectVolume(dir string) error {
	return fmt.Errorf("eject is not supported on this system: unmount %s yourself", dir)
}
