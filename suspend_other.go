// SPDX-License-Identifier: MIT

//go:build !unix

package main

import "github.com/gdamore/tcell/v2"

// suspend does nothing here: there is no job control to return to.
func suspend(tcell.Screen) {}
