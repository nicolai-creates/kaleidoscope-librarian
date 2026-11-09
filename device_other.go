// SPDX-License-Identifier: MIT

//go:build !linux && !darwin && !windows

package main

// findVolume is not implemented on this platform; use KPM_DEVICE.
func findVolume(label string) (string, bool) { return "", false }
