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

// field is a byte range of the preset. Labels are PAGE-PARM as on the synth
// display with firmware v81 (see analysis/FORMAT.md).
type field struct {
	Off, Len int
	Label    string
	Note     string
}

// The parameter area 0x00-0x7f is indexed by MIDI CC number: the byte at
// offset n holds the parameter with CC n (manual, MIDI parameter table).
// CCs without a stored value (unused numbers, and the ALIA parameters, which
// only mirror another parameter) are ff.
//
// slotOff is the PRST-SLOT byte (CC 114).
const slotOff = 0x72

var layout = []field{
	{0x00, 4, "magic", "ff ff ff ff"},
	{0x04, 1, "vSEQ-PSIZ", "pattern size"},
	{0x05, 1, "vSEQ-PATN", "pattern mask"},
	{0x06, 1, "FREQ-FINE", "fine tuning"},
	{0x07, 1, "ADTV-DRIV", "overdrive"},
	{0x08, 1, "vENV-PERI", "attack-decay period"},
	{0x09, 1, "vENV-SHAP", "attack-decay asymmetry"},
	{0x0a, 1, "ADTV-MIDL", "triangle osc asymmetry"},
	{0x0b, 1, "ADTV-MULT", "fold/sync multiplier (FOLD before fw v76)"},
	{0x0c, 1, "ADTV-MIX", "mix"},
	{0x0d, 1, "SUBT-FEDB", "comb feedback"},
	{0x0e, 1, "MAST-FMSR", "FM source tracks"},
	{0x0f, 1, "FREQ-GLID", "glide"},
	{0x10, 1, "MAST-VOLU", "volume"},
	{0x11, 1, "SUBT-FILT", "bit 7 = high-pass, bits 0-6 = cutoff note"},
	{0x12, 1, "SUBT-RESO", "resonance"},
	{0x13, 1, "vENV-SUST", "sustain"},
	{0x14, 1, "mENV-QNT2", "quantize 2"},
	{0x15, 1, "vSEQ-LINK", "EOC link"},
	{0x16, 1, "vSEQ-EOC", "EOC trigger on/off"},
	{0x17, 1, "FREQ-TRAN", "transpose, 0x3f = 0"},
	{0x18, 1, "mENV-SHAP", "attack-decay asymmetry"},
	{0x19, 1, "mENV-PERI", "attack-decay period"},
	{0x1b, 1, "mSEQ-EOC", "EOC trigger on/off"},
	{0x1c, 1, "mSEQ-LINK", "EOC link"},
	{0x1d, 1, "mMX1-DEPT", "modulation depth, 0x40 = 0, wraps"},
	{0x1e, 1, "mMX1-PARM", "parameter (PARM list)"},
	{0x1f, 1, "mMX1-DEST", "destination track, 0-based"},
	{0x20, 1, "SUBT-DELA", "comb delay"},
	{0x21, 1, "mENV-QNT1", "quantize 1"},
	{0x22, 1, "MAST-FMVO", "FM amplitude"},
	{0x23, 1, "vENV-RELE", "release"},
	{0x24, 1, "INTF-TSEL", "track selector mode"},
	{0x26, 1, "INTF-BPM", "tempo in BPM"},
	{0x27, 1, "FREQ-SCAL", "note quantize scale"},
	{0x28, 1, "INTF-WSEL", "scope tracks"},
	{0x29, 1, "INTF-WSYN", "scope sync track"},
	{0x2a, 1, "cc42", "mCH in the manual; always 04"},
	{0x2b, 1, "cc43", "vCH in the manual; always 04"},
	{0x2c, 1, "mMX2-DEPT", "modulation depth, 0x40 = 0, wraps"},
	{0x2d, 1, "mMX2-PARM", "parameter (PARM list)"},
	{0x2e, 1, "mMX2-DEST", "destination track, 0-based"},
	{0x30, 1, "vMX1-DEPT", "modulation depth, 0x40 = 0, wraps"},
	{0x31, 1, "vMX1-PARM", "parameter (PARM list)"},
	{0x32, 1, "vMX1-DEST", "destination track, 0-based"},
	{0x34, 1, "vMX2-DEPT", "modulation depth, 0x40 = 0, wraps"},
	{0x35, 1, "vMX2-PARM", "parameter (PARM list)"},
	{0x36, 1, "vMX2-DEST", "destination track, 0-based"},
	{0x38, 1, "mSEQ-PATN", "pattern mask"},
	{0x39, 1, "mSEQ-PSIZ", "pattern size"},
	{0x3a, 1, "aMX-PARM", "aftertouch: parameter (PARM list)"},
	{0x3b, 1, "aMX-DEST", "aftertouch: destination track, 0-based"},
	{0x3c, 1, "aMX-DEPT", "aftertouch: modulation depth, 0x40 = 0, wraps"},
	{0x3d, 1, "aENV-RISE", "aftertouch: rise smoothing"},
	{0x3e, 1, "aENV-FALL", "aftertouch: fall smoothing"},
	{0x40, 1, "fMX-PARM", "velocity: parameter (PARM list)"},
	{0x41, 1, "fMX-DEST", "velocity: destination track, 0-based"},
	{0x42, 1, "fMX-DEPT", "velocity: modulation depth, 0x40 = 0, wraps"},
	{0x43, 1, "fENV-RISE", "velocity: rise smoothing"},
	{0x44, 1, "fENV-FALL", "velocity: fall smoothing"},
	{0x46, 1, "kMX-PARM", "keytrack: parameter (PARM list)"},
	{0x47, 1, "kMX-DEST", "keytrack: destination track, 0-based"},
	{0x48, 1, "kMX-DEPT", "keytrack: modulation depth, 0x40 = 0, wraps"},
	{0x49, 1, "kENV-RISE", "keytrack: rise smoothing"},
	{0x4a, 1, "kENV-FALL", "keytrack: fall smoothing"},
	{0x4c, 1, "wMX-PARM", "mod wheel: parameter (PARM list)"},
	{0x4d, 1, "wMX-DEST", "mod wheel: destination track, 0-based"},
	{0x4e, 1, "wMX-DEPT", "mod wheel: modulation depth, 0x40 = 0, wraps"},
	{0x50, 1, "pMX-PARM", "pitch bend: parameter (PARM list)"},
	{0x51, 1, "pMX-DEST", "pitch bend: destination track, 0-based"},
	{0x52, 1, "pMX-DEPT", "pitch bend: modulation depth, 0x40 = 0, wraps"},
	{0x54, 1, "MAST-SEND", "audio out tracks"},
	{0x55, 1, "sMX-PARM", "step mod: parameter (PARM list)"},
	{0x56, 1, "sMX-DEST", "step mod: destination track, 0-based"},
	{0x57, 1, "sMX-TSRC", "step mod: 0 OFF, 1 vENV, 2 mENV, 3 smEN?, 4 TSEL?"},
	{0x5a, 1, "sSEQ-LEN", "sequence length 1-16 (set via CURS)"},
	{0x5b, 1, "sSEQ-ROTA", "rotation offset; one click = 0f"},
	{0x5c, 1, "sSEQ-RAND", "randomness 0-32"},
	{0x70, 1, "MIDI-vCH", "vol env channel: 10 + n-1 = mn<n>"},
	{0x71, 1, "MIDI-mCH", "mod env: 00 + n-1 = tr<n>"},
	{slotOff, 1, "PRST-SLOT", "slot the preset was saved from?"},
	{0x74, 1, "MIDI-pCH", "parameter channel: n-1 = mn<n>"},
	{nameOff, nameLen, "name", "1-9,a-z (0 assumed); 00-padded"},
	{0x8c, 16, "sSEQ-VALU", "16 step values, 0x40 + value (-36..36)"},
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
