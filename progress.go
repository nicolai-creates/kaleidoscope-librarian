// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// progressFn is called by bulk operations after each file with the number
// done so far. A nil progressFn reports nothing.
type progressFn func(done int)

func (f progressFn) step(done int) {
	if f != nil {
		f(done)
	}
}

// progressBar draws done of total as a bar of width cells plus a count,
// e.g. "██████░░░░░░  37/128".
func progressBar(done, total, width int) string {
	fill := width
	if total > 0 {
		fill = min(width, done*width/total)
	}
	return fmt.Sprintf("%s%s  %d/%d", strings.Repeat("█", fill), strings.Repeat("░", width-fill), done, total)
}

// throttle calls show at most every 40 ms, but always for the last step.
func throttle(total int, show func(done int)) progressFn {
	var last time.Time
	return func(done int) {
		if done >= total || time.Since(last) >= 40*time.Millisecond {
			last = time.Now()
			show(done)
		}
	}
}

// cliProgress shows a progress line on stderr when it is a terminal, for
// operations with more than a few files. The line ends when all are done.
func cliProgress(label string, total int) progressFn {
	if total < 10 || !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil
	}
	return throttle(total, func(done int) {
		fmt.Fprintf(os.Stderr, "\r%s %s", label, progressBar(done, total, 30))
		if done >= total {
			fmt.Fprintln(os.Stderr)
		}
	})
}
