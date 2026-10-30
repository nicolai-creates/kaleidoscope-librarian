# Kaleidoscope single-voice preset (impNNN.bin), 256 bytes

Source: `Factory Presets/` (39 presets, plus an empty `imp.bin`: an import marker that tells the synth to import the presets in the folder; the synth deletes it after importing).

Generated files, all produced by `kpm` from the layout in `format.go`:
- `presets.json`: `kpm export json "Factory Presets"` (layout, then every preset with its fields and tail)
- `presets.csv`: `kpm export csv "Factory Presets"` (one row per preset, one column per byte, magic and name excluded)
- `stats.txt`: `kpm stats "Factory Presets"` (per-byte distinct values / min / max / most common values, and tail groups)

Re-run these after changing the layout. Offsets are hex. "Slot" is the number in the filename. Usable slots are 1-126; slot 127 is the init preset and always loads as an empty preset on the synth.

A preset holds the parameters of one track (the synth has 4). The user manual (KALEIDOSCOPE user manual, EN, July 2025) describes the parameters but not the file format. It is not part of this repository.

## Overview
| Offset | Content |
|---|---|
| 0x00-0x03 | magic `ff ff ff ff` (CC 0-3 are unused) |
| 0x04-0x7f | parameters, indexed by MIDI CC number |
| 0x80-0x8a | name |
| 0x8b-0xff | tail, probably not preset data |

## Parameters are indexed by MIDI CC number
The manual's MIDI parameter table gives every parameter a CC number. **The byte at offset n holds the parameter with CC n.** This explains every byte of the old positional layout:
- Every `ff` byte in 0x04-0x7f (identical in all 39 files) is a CC that stores no value: the ALIA parameters of the nine modulation matrices (CC 37, 47, 51, 55, 63, 69, 75, 79, 83; ALIA only mirrors the modulated parameter), and the unused CCs 26, 85-111, 115, 117-127. Firmware v81 also writes 85-92 (see Device tests).
- Every other byte in that range is a parameter from the table, except 0x70 and 0x71 (CC 112, 113), which the manual does not list.
- The two "records" of the earlier analysis are the modulation matrices. The CC order puts DEPT first in vMX/mMX (`DEPT, PARM, DEST`) and last in aMX/fMX/kMX/wMX/pMX (`PARM, DEST, DEPT`). The 5-byte records are the aftertouch, velocity and keytrack matrices followed by RISE and FALL of their smoothing page; mod wheel and pitch bend have no smoothing page, hence 3 bytes.
- Values sit where the manual's ranges put the midpoint: MIDL `40` (0.5), every DEPT `40` (0), TRAN and FINE `3f`, mENV-SHAP `40`. BPM is `78` = 120 in 33 files, `5a` = 90, `8c` = 140. Both pattern fields default to PSIZ `07` (8 steps, 0-based) with PATN `ff` (all steps on).

Labels are `PAGE-PARM` as on the synth display with firmware v81, which renamed FOLD to MULT (v76) and SBTV to SUBT (v80). The pages are grouped as on the page selector (8 groups of 4 pages; group 7 holds the step modulator since v76, group 8 is empty):

| Group | Pages |
|---|---|
| Waveform | ADTV, SUBT (SBTV in the manual), FREQ, MAST |
| Volume envelope | vENV, vSEQ, vMX1, vMX2 |
| Modulation envelope | mENV, mSEQ, mMX1, mMX2 |
| Configuration | MIDI, INTF, PRST, SCOP |
| External modulators 1 | aENV, aMX, fENV, fMX |
| External modulators 2 | kENV, kMX, wMX, pMX |
| Step modulator (v76) | sSEQ, sMX |

| Offset | CC | Label | Observed in the factory presets |
|---|---|---|---|
| 0x04 | 4 | vSEQ-PSIZ | `07` in 35 files; `02` motor, aliengirl; `03` acidrave3; `04` punchface |
| 0x05 | 5 | vSEQ-PATN | `ff` in 35 files; `0f`, `1b`, `15` in the same four |
| 0x06 | 6 | FREQ-FINE | 31-116, `3f` in 14 |
| 0x07 | 7 | ADTV-DRIV | 29-94 |
| 0x08 | 8 | vENV-PERI | 24-98 |
| 0x09 | 9 | vENV-SHAP | 0-84, `00` in 26 |
| 0x0a | 10 | ADTV-MIDL | 2-127, `40` in 11 |
| 0x0b | 11 | ADTV-MULT | 0-127 |
| 0x0c | 12 | ADTV-MIX | 0-127, `40` in 11 |
| 0x0d | 13 | SUBT-FEDB | 36-99 |
| 0x0e | 14 | MAST-FMSR | `01` (23), `00` (15), `26` pondfish |
| 0x0f | 15 | FREQ-GLID | 0-115 |
| 0x10 | 16 | MAST-VOLU | `7f` in 34 |
| 0x11 | 17 | SUBT-FILT | 0-201; bit 7 = high-pass, bits 0-6 = note (164 = C2 high-pass) |
| 0x12 | 18 | SUBT-RESO | 0-56 |
| 0x13 | 19 | vENV-SUST | 0-91 |
| 0x14 | 20 | mENV-QNT2 | `00`, `0c` (12 = octaves), `10` |
| 0x15 | 21 | vSEQ-LINK | `01` in 38, `24` pondfish |
| 0x16 | 22 | vSEQ-EOC | `00` / `01` |
| 0x17 | 23 | FREQ-TRAN | 0-80, `3f` in 22 |
| 0x18 | 24 | mENV-SHAP | 0-108, `40` in 24 |
| 0x19 | 25 | mENV-PERI | 36-106 |
| 0x1b | 27 | mSEQ-EOC | `01` / `00` |
| 0x1c | 28 | mSEQ-LINK | `01` in 38, `24` pondfish |
| 0x1d | 29 | mMX1-DEPT | 34-131 |
| 0x1e | 30 | mMX1-PARM | 0-16 |
| 0x1f | 31 | mMX1-DEST | `00` in 38, `0a` pondfish |
| 0x20 | 32 | SUBT-DELA | 0-59 |
| 0x21 | 33 | mENV-QNT1 | 0-19 |
| 0x22 | 34 | MAST-FMVO | 16-112, `40` in 13 |
| 0x23 | 35 | vENV-RELE | 0-103 |
| 0x24 | 36 | INTF-TSEL | `00` (HOLD), `01` (DRONE) motor, aliengirl |
| 0x26 | 38 | INTF-BPM | 120, 90, 140 |
| 0x27 | 39 | FREQ-SCAL | `00` (26), `0f` (10), `01`-`03` |
| 0x28 | 40 | INTF-WSEL | `0f` (35), `ff` (4) |
| 0x29 | 41 | INTF-WSYN | `00` (33), `ff` (4), `03` (2) |
| 0x2a | 42 | cc42 | `04` in every file and in the v81 exports; the manual's CC for mCH, but the channel is stored at 0x71 |
| 0x2b | 43 | cc43 | `04` everywhere; the manual's CC for vCH, stored at 0x70 |
| 0x2c-0x2e | 44-46 | mMX2-DEPT, PARM, DEST | |
| 0x30-0x32 | 48-50 | vMX1-DEPT, PARM, DEST | |
| 0x34-0x36 | 52-54 | vMX2-DEPT, PARM, DEST | |
| 0x38 | 56 | mSEQ-PATN | `ff` in 37; `07` acidrave3, `0f` punchface |
| 0x39 | 57 | mSEQ-PSIZ | `07` in 37; `04` acidrave3, `02` punchface |
| 0x3a-0x3e | 58-62 | aMX-PARM, DEST, DEPT, aENV-RISE, FALL | default `00 00 40 3c 3c` |
| 0x40-0x44 | 64-68 | fMX-PARM, DEST, DEPT, fENV-RISE, FALL | default `00 00 40 3c 3c` |
| 0x46-0x4a | 70-74 | kMX-PARM, DEST, DEPT, kENV-RISE, FALL | default `00 00 40 3c 3c` |
| 0x4c-0x4e | 76-78 | wMX-PARM, DEST, DEPT | default `00 00 40` |
| 0x50-0x52 | 80-82 | pMX-PARM, DEST, DEPT | default `00 00 40` |
| 0x54 | 84 | MAST-SEND | `0f` in every file (all four outputs) |
| 0x70 | 112 | MIDI-vCH | `10` + channel-1, displayed `mn<n>`; `10` in 38, `12` pondfish |
| 0x71 | 113 | MIDI-mCH | track-1, displayed `tr<n>` (`00` = tr1); `04` in 36, `14` l2 and lostinspacy, `06` pondfish (meaning of these not known) |
| 0x72 | 114 | PRST-SLOT | unique per file, 1-71 |
| 0x74 | 116 | MIDI-pCH | channel-1, displayed `mn<n>`; `04` in 38, `06` pondfish |

motor, punchface and aliengirl share one non-default modulation setup: PARM `1f` (31) in aMX, fMX, kMX, wMX and pMX, with aENV and fENV smoothing `24`/`18` instead of `3c`.

## Device tests (firmware v81)
Test files in `refactoring-01/` and `refactoring-02/`, exported from a Kaleidoscope with firmware v81. They are built on the factory presets and, like those, not part of the repository. The changelog is at https://github.com/vadjuse/kaleidoscope/blob/main/Release%20FWs/changelog.txt.

| File | How it was made |
|---|---|
| exp100.bin | spacehunt loaded into track 3 and saved unchanged; tracks set to channel = track number |
| exp101.bin | spacehunt loaded into track 2 and saved unchanged |
| exp102.bin | empty patch, then DEST, LINK, FMSR, SEND, SCAL, TSEL, BPM changed |
| exp103.bin | empty patch, then parameters set to their minimum or maximum |
| exp104.bin | empty patch, step modulator: length 5, steps +5/-5, RAND 7, sMX PARM FILT, DEST tr2, TSRC mENV |
| exp105.bin | as exp104, then ROTA turned one click |

Confirmed:
- PRST-SLOT is the slot the preset was saved to (100-103).
- vMX/mMX DEST is the track, 0-based: displayed 2, 3, 4 are stored as `01`, `02`, `03`.
- LINK, FMSR and SEND are track bit masks, bit 0 = track 1: tracks 1+3 = `05`, track 2 = `02`, output 1 = `01`.
- TSEL EDIT = `02`, SCAL nMAJ = `01`, BPM is stored as is (200 = `c8`).
- Minimum is `00` and maximum `7f` for MIDL, vENV-PERI, FILT, RESO, TRAN, mENV-PERI and GLID.
- TRAN: displayed value + 63 (12 = `4b`, the empty patch default; maximum = `7f`).
- DEPT: `40` + depth, wrapping at 8 bits. Display minimum -128 = `c0`, maximum 128 = `bf`. This explains the factory values above 127.
- FILT: bit 7 selects high-pass (filter types are selectable since v76). spacehunt `a4` shows as `C2h`: 164 - 128 = 36 = C2.

The channel bytes identify the track a preset was saved from: vCH `12`, mCH `02`, pCH `02` in exp100 (track 3), `11`, `01`, `01` in exp101 (track 2). (The notes taken during the test had 100 and 101 the other way round; the bytes are consistent only this way.)

Loading spacehunt into track 2 or 3 makes all its track references point to that track (the display showed DEST = own track). On save they are stored as DEST `05` (track 2) / `0a` (track 3) and LINK, FMSR `12` / `24`. The low 2 bits of DEST and the low 4 bits of LINK/FMSR give the track; the extra high bits are not understood. pondfish has the same values as exp100, so it was probably saved from track 3.

Bytes written by v81 but `ff` or tail in the factory presets (step modulator, added in v76, has no CC numbers in the manual):
| Offset | Label | Encoding (confirmed with exp104/105) |
|---|---|---|
| 0x55 | sMX-PARM | see PARM list |
| 0x56 | sMX-DEST | track, 0-based |
| 0x57 | sMX-TSRC | 0 OFF, 1 vENV, 2 mENV; smEN and TSEL presumably 3 and 4 |
| 0x58, 0x59 | - | `ff`: presumably sMX-ALIA and sSEQ-VALU, which have no single value |
| 0x5a | sSEQ-LEN | sequence length 1-16 (set by holding CURS) |
| 0x5b | sSEQ-ROTA | rotation offset; one click = `0f`. Rotating does not move the step bytes |
| 0x5c | sSEQ-RAND | 0-32 |
| 0x8c-0x9b | sSEQ-VALU | 16 steps, `40` + value (-36..36) |

### PARM list
All PARM fields (vMX1/2, mMX1/2, aMX, fMX, kMX, wMX, pMX, sMX) select the modulated parameter by index. The order as shown on the display with v81; the index is confirmed for sMX (FILT = 5):

| Index | Parameter | Index | Parameter | Index | Parameter | Index | Parameter |
|---|---|---|---|---|---|---|---|
| 0 | EMPT | 8 | FEDB | 16 | vPTN | 24 | mPTN |
| 1 | MIDL | 9 | FREQ | 17 | vPS2 | 25 | mPS2 |
| 2 | MULT | 10 | FINE | 18 | vDP1 | 26 | mDP1 |
| 3 | MIX | 11 | VOLU | 19 | vDP2 | 27 | mDP2 |
| 4 | DRIV | 12 | FMVO | 20 | mSHP | 28 | GLID |
| 5 | FILT | 13 | FMSR | 21 | mPER | 29 | RELE |
| 6 | RESO | 14 | vSHP | 22 | mQN1 | 30 | EMPT |
| 7 | DELA | 15 | vPER | 23 | mQN2 | 31 | EMPT |

GLID and RELE were added as destinations in v76. The factory presets use indices up to 22, plus 31 in the aMX/fMX/kMX/wMX/pMX of motor, punchface and aliengirl. 31 is EMPT in v81, and motor loaded on v81 shows EMPT there too, so these slots are simply unused. The manual gives kMX, wMX and pMX a range of 1-16 only.

The cursor position is not stored. Presets saved before v76 have tail bytes at 0x8c-0x9b and `ff` at 0x55-0x5c, so an old preset may load with random step values.

### PRST-SLOT (0x72), no checksum
The byte is CC 114, PRST-SLOT. It is the slot the preset was saved to (confirmed on the device), not the slot in the filename: `glitch2`..`glitch8` (file slots 26-32) hold 53, 55-60, and 54 is `oldbastard`. No byte behaves like a checksum, so a rename (0x80-0x8a only) needs no other change. One rename test on the device is still worthwhile.

### pondfish
pondfish (slot 16) is the only file with DEST `0a` in all nine matrices, LINK `24` in both sequencers, pCH `06` and cc112 `12`. It may have been saved from a different track or with older firmware.

## Name (0x80-0x8a)
11 bytes, `00`-padded. Not null-terminated at 11 chars (`lostinspacy`).

Name charset: `01`-`09` = `1`-`9`, `0b`-`24` = `a`-`z`. Lowercase, no spaces or punctuation.
Check: all 39 names decode to readable words, `glitch2`..`glitch8` fall in consecutive slots 26-32, and `z` = `24` is confirmed by `lazermercy`.
Not covered by the factory names: `0`, `j`, `q`, `x`. `j`/`q`/`x` follow from the a-z run. `0` = `0a` is an assumption (it fills the gap after `9` = `09`) and should be tested on the device.

## Tail (0x8b-0xff)
- 3 files are all zero (slots 1, 2, 25).
- 11 files are zero except for a small block. 10 of them (slots 18, 19, 26, 28-32, 37, 38) have the same 12 bytes at 0xad-0xb8: `1f 01 12 21 21 12 23 41 12 25 f1 ff`. All 10 have PRST-SLOT 51-64. spacehunt (slot 3) has a different block at 0x98-0xa6: `11 21 11 13 41 11 15 61 11 17 f1 ff ff ff ff`.
- 25 files hold one of 7 variants of the same noise-like 117-byte pattern (starting `e3|e6|e7 3b|bb d1 8x`). The variants differ only by scattered bit flips, which looks like uninitialised RAM.

The tail is probably leftover buffer content and not part of the preset. The small blocks have a nibble pattern (`x1 y2 ...`) and might be a fragment of some other structure.

## Open questions
The current analysis is based on the factory presets and the user manual alone. Raw values have not yet been checked against values set on the device.

- Value scaling of PERI, RELE, GLID and the other time parameters.
- The rule behind the track-dependent values (`05`/`0a`, `12`/`24`) when saving from tracks 2-4.
- What 0x2a/0x2b (`04` everywhere) are.
- The high bits of DEST/LINK/FMSR after loading into another track.
- How vCH encodes polyphonic channels, and mCH `04`/`14` in the factory presets.
- Whether the other PARM fields use the same numbering as sMX (very likely), and whether it changed between firmware versions.
- TSRC codes for smEN and TSEL.
- Does `0a` really encode `0` in names?

Controlled experiments answer most of these: change parameters to known displayed values on the device, save, export, diff.
