// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Info rendering (shared by CLI and TUI)

func summaryLines(p *Preset) []string {
	l := []string{"file   " + p.File()}
	if !p.Valid() {
		return append(l, "error  "+p.Err.Error())
	}
	slot := "-"
	if s, _, ok := p.Slot(); ok {
		slot = fmt.Sprintf("%03d", s)
		if !slotOK(s) {
			slot += fmt.Sprintf(" (outside %d-%d, not loadable on the synth)", minSlot, maxSlot)
		}
	}
	nameRaw := hexBytes(p.Data[nameOff : nameOff+nameLen])
	magicState := "ok"
	if !p.MagicOK() {
		magicState = "BAD " + hexBytes(p.Data[:4])
	}
	sufState := map[string]string{"": "none", "=": "matches name", "!": "differs from name"}[p.SuffixState()]
	l = append(l,
		"slot   "+slot,
		"name   "+p.Name(),
		"raw    "+nameRaw,
		"magic  "+magicState,
		"fname  "+sufState,
		fmt.Sprintf("slot?  0x%02x = %02x (%d), PRST-SLOT", slotOff, p.Data[slotOff], p.Data[slotOff]),
	)
	return l
}

func hexBytes(b []byte) string {
	s := make([]string, len(b))
	for i, c := range b {
		s[i] = fmt.Sprintf("%02x", c)
	}
	return strings.Join(s, " ")
}

func decBytes(b []byte) string {
	s := make([]string, len(b))
	for i, c := range b {
		s[i] = fmt.Sprintf("%3d", c)
	}
	return strings.Join(s, " ")
}

// fieldBlocks returns, per layout field, a header line and value lines with
// perRow bytes each. A single-byte field is one line: label, offset, value
// and note.
func fieldBlocks(p *Preset, dec bool, perRow int) [][]string {
	var out [][]string
	for _, f := range layout {
		b := p.Data[f.Off : f.Off+f.Len]
		if f.Len == 1 {
			v := hexBytes(b) + " "
			if dec {
				v = decBytes(b)
			}
			out = append(out, []string{fmt.Sprintf("%-9s 0x%02x  %s  %s", f.Label, f.Off, v, f.Note)})
			continue
		}
		block := []string{fmt.Sprintf("%-9s 0x%02x-0x%02x  %s", f.Label, f.Off, f.Off+f.Len-1, f.Note)}
		for i := 0; i < len(b); i += perRow {
			row := b[i:min(i+perRow, len(b))]
			if dec {
				block = append(block, "          "+decBytes(row))
			} else {
				block = append(block, "          "+hexBytes(row))
			}
		}
		if f.Label == "name" {
			block[len(block)-1] += "   \"" + p.Name() + "\""
		}
		out = append(out, block)
	}
	return out
}
