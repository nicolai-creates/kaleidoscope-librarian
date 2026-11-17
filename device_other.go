// SPDX-License-Identifier: MIT

//go:build !linux && !darwin && !windows

package main

import "fmt"

// findVolume is not implemented on this platform; use KALIB_DEVICE.
func findVolume(label string) (string, bool) { return "", false }

// ejectVolume is not implemented on this platform.
func ejectVolume(dir string) error {
	return fmt.Errorf("eject is not supported on this system: unmount %s yourself", dir)
}

// unmountedVolume is not needed here: the system mounts the drive itself.
func unmountedVolume(label string) (string, bool) { return "", false }

// mountVolume is not implemented on this platform.
func mountVolume(dev string) error {
	return fmt.Errorf("mounting is not supported on this system: mount %s yourself", dev)
}
