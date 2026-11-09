// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
)

// The synth shows up as a USB drive with this volume label.
const deviceLabel = "KLRU"

// deviceEnv overrides the search with the folder of the synth's drive.
const deviceEnv = "KPM_DEVICE"

var errNoDevice = fmt.Errorf("synth not found: no drive named %s is mounted (set %s to its folder)", deviceLabel, deviceEnv)

// findDevice returns the folder where the synth's drive is mounted. The
// search itself is per platform (findVolume in device_<os>.go).
func findDevice() (string, error) {
	if dir := os.Getenv(deviceEnv); dir != "" {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return "", fmt.Errorf("%s=%s is not a folder", deviceEnv, dir)
		}
		return dir, nil
	}
	dir, ok := findVolume(deviceLabel)
	if !ok {
		return "", errNoDevice
	}
	return dir, nil
}
