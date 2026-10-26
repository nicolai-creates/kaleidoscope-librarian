// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Format

// Usable preset slots. Slot 127 is the init preset: it always loads as an
// empty preset on the synth.
const (
	minSlot = 1
	maxSlot = 126
)

const (
	presetSize = 256
	nameOff    = 0x80
	nameLen    = 11
)

var magic = []byte{0xff, 0xff, 0xff, 0xff}

// field is a byte range of the preset. Labels are positional until the
// meaning of a range is known (see analysis/FORMAT.md).
type field struct {
	Off, Len int
	Label    string
	Note     string
}

// Record types (see FORMAT.md):
//
//	A: 3 bytes (value ~40, 00-16, flag 00|0a)
//	B: 5 bytes (sel 00|1f|.., flag 00|0a, 40|bf, x, y); i and j hold only the first 3
var layout = []field{
	{0x00, 4, "magic", "ff ff ff ff"},
	{0x04, 2, "header", "07 ff in 35 files"},
	{0x06, 20, "sec_a", "high variance: osc / filter / main?"},
	{0x1b, 2, "sec_b", ""},
	{0x1d, 3, "rec_b", "record A"},
	{0x20, 5, "sec_b2", ""},
	{0x26, 6, "sec_c", "0x2a-0x2b always 04"},
	{0x2c, 3, "rec_c", "record A"},
	{0x30, 3, "rec_d", "record A"},
	{0x34, 3, "rec_e", "record A"},
	{0x38, 2, "pair_f", "ff 07 in 37 files"},
	{0x3a, 5, "rec_f", "record B; default 00 00 40 3c 3c"},
	{0x40, 5, "rec_g", "record B; default 00 00 40 3c 3c"},
	{0x46, 5, "rec_h", "record B; default 00 00 40 3c 3c"},
	{0x4c, 3, "rec_i", "record B (short); default 00 00 40"},
	{0x50, 3, "rec_j", "record B (short); default 00 00 40"},
	{0x54, 1, "sec_k", "0f in every file"},
	{0x70, 2, "sec_l", "10 04 in most files"},
	{0x72, 1, "seq", "save sequence number? unique per preset"},
	{0x74, 1, "sec_m", "04 in most files"},
	{nameOff, nameLen, "name", "1-9,a-z (0 assumed); 00-padded"},
}

// fieldAt maps each byte offset to its index in layout, or -1.
var fieldAt = func() (m [presetSize]int) {
	for i := range m {
		m[i] = -1
	}
	for i, f := range layout {
		for o := f.Off; o < f.Off+f.Len; o++ {
			if m[o] >= 0 {
				panic(fmt.Sprintf("layout: %s overlaps %s at 0x%02x", f.Label, layout[m[o]].Label, o))
			}
			m[o] = i
		}
	}
	return
}()

// Name charset: 01-09 = '1'-'9', 0a = '0', 0b-24 = 'a'-'z', 00 = padding.
func codeToChar(c byte) (rune, bool) {
	switch {
	case c >= 0x01 && c <= 0x09:
		return rune('0' + c), true
	case c == 0x0a:
		return '0', true
	case c >= 0x0b && c <= 0x24:
		return rune('a' + c - 0x0b), true
	}
	return 0, false
}

func charToCode(r rune) (byte, bool) {
	switch {
	case r >= '1' && r <= '9':
		return byte(r - '0'), true
	case r == '0':
		return 0x0a, true
	case r >= 'a' && r <= 'z':
		return byte(r-'a') + 0x0b, true
	}
	return 0, false
}

func decodeName(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c == 0 {
			break
		}
		if r, ok := codeToChar(c); ok {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('?')
		}
	}
	return sb.String()
}

func encodeName(s string) ([]byte, error) {
	s = strings.ToLower(s)
	if s == "" {
		return nil, errors.New("name is empty")
	}
	if len(s) > nameLen {
		return nil, fmt.Errorf("name %q longer than %d characters", s, nameLen)
	}
	out := make([]byte, nameLen)
	for i, r := range s {
		c, ok := charToCode(r)
		if !ok {
			return nil, fmt.Errorf("invalid character %q in name (allowed: a-z, 0-9)", r)
		}
		out[i] = c
	}
	return out, nil
}

// sanitizeName turns arbitrary text (e.g. a filename suffix) into a valid
// preset name: lowercase, invalid characters dropped, truncated.
func sanitizeName(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if _, ok := charToCode(r); ok && sb.Len() < nameLen {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
