// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// Control codes from winioctl.h (not in x/sys/windows).
const (
	fsctlLockVolume          = 0x00090018
	fsctlDismountVolume      = 0x00090020
	ioctlStorageMediaRemoval = 0x002d4804
	ioctlStorageEjectMedia   = 0x002d4808
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

// ejectVolume flushes, locks and dismounts the volume, then ejects the
// media: the usual sequence for removable drives, no admin rights needed.
func ejectVolume(dir string) error {
	vol := filepath.VolumeName(dir) // "E:"
	if vol == "" {
		return fmt.Errorf("%s: not on a drive letter", dir)
	}
	h, err := windows.CreateFile(windows.StringToUTF16Ptr(`\\.\`+vol),
		windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return fmt.Errorf("open %s: %v", vol, err)
	}
	defer windows.CloseHandle(h)
	var n uint32
	ioctl := func(code uint32, in []byte) error {
		var p *byte
		if len(in) > 0 {
			p = &in[0]
		}
		return windows.DeviceIoControl(h, code, p, uint32(len(in)), nil, 0, &n, nil)
	}
	if err := windows.FlushFileBuffers(h); err != nil {
		return fmt.Errorf("flush %s: %v", vol, err)
	}
	// Locking fails while files are open; other programs may need a moment.
	for i := 0; ; i++ {
		if err = ioctl(fsctlLockVolume, nil); err == nil {
			break
		}
		if i == 10 {
			return fmt.Errorf("%s is in use (close programs using it): %v", vol, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err := ioctl(fsctlDismountVolume, nil); err != nil {
		return fmt.Errorf("dismount %s: %v", vol, err)
	}
	allow := []byte{0} // PREVENT_MEDIA_REMOVAL{PreventMediaRemoval: false}
	if err := ioctl(ioctlStorageMediaRemoval, allow); err != nil {
		return fmt.Errorf("allow removal of %s: %v", vol, err)
	}
	if err := ioctl(ioctlStorageEjectMedia, nil); err != nil {
		return fmt.Errorf("eject %s: %v", vol, err)
	}
	return nil
}
