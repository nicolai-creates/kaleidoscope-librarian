// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Info rendering (shared by CLI and TUI)

// summaryLines is the header of a preset, "018 · iamwhoami    (imp018_iamwhoami.bin)"
// with the name padded to its maximum length so the filename stays in place,
// then a line per problem (error, request, name mismatch, slot, magic).
func summaryLines(p *Preset) []string {
	slot := "---"
	s, _, ok := p.Slot()
	if ok {
		slot = fmt.Sprintf("%03d", s)
	}
	l := []string{fmt.Sprintf("%s · %-*s  (%s)", slot, nameLen, p.Name(), p.File())}
	switch {
	case p.Request():
		return append(l, "export request: the synth writes this slot into the file")
	case !p.Valid():
		return append(l, "error: "+p.Err.Error())
	}
	if p.SuffixState() == "!" {
		l = append(l, "name in filename differs")
	}
	switch {
	case !ok:
	case !slotOK(s):
		l = append(l, fmt.Sprintf("slot outside %d-%d, not loadable on the synth", minSlot, maxSlot))
	case s == initSlot:
		l = append(l, "init preset slot: loads as an empty preset on the synth")
	}
	if !p.MagicOK() {
		l = append(l, "magic BAD: "+hexBytes(p.Data[:4]))
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
