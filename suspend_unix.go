// SPDX-License-Identifier: MIT

//go:build unix

package main

import (
	"syscall"

	"github.com/gdamore/tcell/v2"
)

// suspend hands the terminal back and stops kalib like the shell's Ctrl-Z
// would (the screen is in raw mode, so the shell never sees the key). It
// returns after fg.
func suspend(s tcell.Screen) {
	if s.Suspend() != nil {
		return
	}
	_ = syscall.Kill(0, syscall.SIGTSTP)
	_ = s.Resume()
}
