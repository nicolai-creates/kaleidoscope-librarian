// SPDX-License-Identifier: MIT

package main

import (
	"errors"
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

// ejectWarning is returned by ejectVolume when the drive is safe to unplug
// but a last step failed (e.g. powering off a drive that cannot be).
type ejectWarning struct{ error }

// ejectDevice flushes and ejects the drive holding dir, so the synth sees
// complete files (ejectVolume in device_<os>.go). A non-empty note is a
// warning about an eject that still succeeded.
func ejectDevice(dir string) (note string, err error) {
	err = ejectVolume(dir)
	var w ejectWarning
	if errors.As(err, &w) {
		return w.Error(), nil
	}
	return "", err
}
