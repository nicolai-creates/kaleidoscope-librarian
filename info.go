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
	if p.Request() {
		s, _, _ := p.Slot()
		return append(l, fmt.Sprintf("state  export request: the synth writes slot %03d into this file", s))
	}
	if !p.Valid() {
		return append(l, "error  "+p.Err.Error())
	}
	// Two lines: name with its file, slot with the slot it was saved to. Name
	// and slot are padded to the name's maximum length so the filename and
	// saved slot stay in place.
	// The filename state only gets a mention when it disagrees.
	name := fmt.Sprintf("name   %-*s  ·  %s", nameLen, p.Name(), p.File())
	if p.SuffixState() == "!" {
		name += "  (name in filename differs)"
	}
	slot, note := "-", ""
	if s, _, ok := p.Slot(); ok {
		slot = fmt.Sprintf("%03d", s)
		if p.Kind() == "exp" {
			note += "  (exported from the synth)"
		}
		switch {
		case !slotOK(s):
			note += fmt.Sprintf("  (outside %d-%d, not loadable on the synth)", minSlot, maxSlot)
		case s == initSlot:
			note += "  (init preset slot: loads as an empty preset on the synth)"
		}
	}
	slot = fmt.Sprintf("slot   %-*s  ·  saved to %d%s", nameLen, slot, p.Data[slotOff], note)
	l = append(l[:0], name, slot)
	if !p.MagicOK() { // only worth a line when wrong
		l = append(l, "magic  BAD "+hexBytes(p.Data[:4]))
	}
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
