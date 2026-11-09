// SPDX-License-Identifier: MIT

package main

import (
	"strings"

	"golang.org/x/sys/windows"
)

// findVolume finds the drive letter whose volume has the given label.
func findVolume(label string) (string, bool) {
	// No "insert a disk" dialogs for empty card readers.
	old := windows.SetErrorMode(windows.SEM_FAILCRITICALERRORS)
	defer windows.SetErrorMode(old)
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return "", false
	}
	for i := 0; i < 26; i++ {
		if drives&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		buf := make([]uint16, windows.MAX_PATH+1)
		err := windows.GetVolumeInformation(windows.StringToUTF16Ptr(root), &buf[0], uint32(len(buf)), nil, nil, nil, nil, 0)
		if err == nil && strings.EqualFold(windows.UTF16ToString(buf), label) {
			return root, true
		}
	}
	return "", false
}
