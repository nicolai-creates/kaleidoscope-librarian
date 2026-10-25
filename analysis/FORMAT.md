# Kaleidoscope single-voice preset (impNNN.bin), 256 bytes

Source: `Factory Presets/` (39 presets, plus an empty `imp.bin`: an import marker that tells the synth to import the presets in the folder; the synth deletes it after importing).

Generated files, all produced by `kpm` from the layout in `main.go`:
- `presets.json`: `kpm export json "Factory Presets"` (layout, then every preset with its fields and tail)
- `presets.csv`: `kpm export csv "Factory Presets"` (one row per preset, one column per byte, magic and name excluded)
- `stats.txt`: `kpm stats "Factory Presets"` (per-byte distinct values / min / max / most common values, and tail groups)

Re-run these after changing the layout. Offsets are hex. "Slot" is the number in the filename.

## Confirmed
| Offset | Field | Notes |
|---|---|---|
| 0x00-0x03 | magic | `ff ff ff ff` in all 39 files |
| 0x80-0x8a | **name** | 11 bytes, `00`-padded. Not null-terminated at 11 chars (`lostinspacy`) |

Name charset: `01`-`09` = `1`-`9`, `0b`-`24` = `a`-`z`. Lowercase, no spaces or punctuation.
Check: all 39 names decode to readable words, `glitch2`..`glitch8` fall in consecutive slots 26-32, and `z` = `24` is confirmed by `lazermercy`.
Not covered by the factory names: `0`, `j`, `q`, `x`. `j`/`q`/`x` follow from the a-z run. `0` = `0a` is an assumption (it fills the gap after `9` = `09`) and should be tested on the device.

## Separators
These bytes are `ff` in every file: 0x1a, 0x25, 0x2f, 0x33, 0x37, 0x3f, 0x45, 0x4b, 0x4f, 0x53, 0x55-0x6f, 0x73, 0x75-0x7f.

## Layout (structure verified, meaning not)
Labels are positional, not functional.

| Field | Offset | Observation |
|---|---|---|
| header | 0x04-0x05 | `07 ff` in 35 files; `02 0f` (motor, aliengirl), `03 1b` (acidrave3), `04 15` (punchface) |
| sec_a | 0x06-0x19 | 20 bytes, high variance: probably oscillator / filter / main sound parameters |
| sec_b | 0x1b-0x1c | 0x1c is `01` except pondfish (`24`) |
| rec_b | 0x1d-0x1f | record A |
| sec_b2 | 0x20-0x24 | 0x24 is `00` except motor and aliengirl (`01`) |
| sec_c | 0x26-0x2b | 0x26 is 120 / 90 / 140; 0x2a-0x2b are `04` in every file |
| rec_c, rec_d, rec_e | 0x2c-0x2e, 0x30-0x32, 0x34-0x36 | record A |
| pair_f | 0x38-0x39 | `ff 07` in 37 files; `07 04` (acidrave3), `0f 02` (punchface). Both also have a non-standard header |
| rec_f, rec_g, rec_h | 0x3a-0x3e, 0x40-0x44, 0x46-0x4a | record B |
| rec_i, rec_j | 0x4c-0x4e, 0x50-0x52 | record B, first 3 bytes only |
| sec_k | 0x54 | `0f` in every file |
| sec_l | 0x70-0x71 | `10 04` in most files; 0x70 = `12` in pondfish, 0x71 = `14` in l2 and lostinspacy, `06` in pondfish |
| seq | 0x72 | unique per file, 1-71: probably a save sequence number (see below) |
| sec_m | 0x74 | `04`, except pondfish (`06`) |
| tail | 0x8b-0xff | not preset data (see below) |

### Records
Two record shapes repeat. They are easiest to see in pondfish (slot 16), the only file in which every flag byte is `0a`. In all other files the flag is `00`.

- **Record A**, 3 bytes, `(value, small, flag)`: value often `40` (range 0x00-0x97), small 0-22, flag `00`|`0a`. Found at 0x1d, 0x2c, 0x30, 0x34.
- **Record B**, 5 bytes, `(sel, flag, mid, x, y)`: sel `00` by default (`1f`, `05`, `07`, `01` seen), flag `00`|`0a`, mid `40` (rec_f also `bf`; rec_h also `bf`, `2c`, `2e`), x/y `3c 3c` by default. In rec_g, rec_i and rec_j mid is `40` in every file. Found at 0x3a, 0x40, 0x46; rec_i and rec_j at 0x4c and 0x50 have only the first 3 bytes.

Default record B set: `00 00 40 3c 3c / 00 00 40 3c 3c / 00 00 40 3c 3c / 00 00 40 / 00 00 40`.
motor, punchface and aliengirl (slots 20, 35, 39) share one identical non-default set: `1f 00 40 24 24 / 1f 00 40 18 18 / 1f 00 40 21 1f / 1f 00 40 / 1f 00 40`. These look like modulation slots (sel = source?).

### seq (0x72) is a counter, not a checksum
In `glitch2`..`glitch8` (slots 26-32) 0x72 runs 53, 55, 56, 57, 58, 59, 60, and the gap at 54 is `oldbastard`. A checksum would not step by one while the parameter bytes change arbitrarily. Renaming a preset (which changes only 0x80-0x8a) therefore probably needs no checksum update. One rename test on the device is still worthwhile.

### Value ranges
Most parameter bytes stay within 0-127. Values above 127 (excluding the `ff` bytes) occur at 0x11 (up to 201), 0x1d, 0x26, 0x30, 0x34, 0x3c and 0x48 (`bf`), so not every parameter is 7-bit.

### Glitch series
The glitch presets are close variants of each other. Bytes that differ between them: 0x06, 0x07, 0x08, 0x0a, 0x0b, 0x0d, 0x0f, 0x17, 0x18, 0x19, 0x1d, 0x1e, 0x20, 0x21, 0x23, 0x30, 0x31, plus 0x72 (seq) and 0x86 (the digit in the name). 0x06, 0x08, 0x0a, 0x17 and 0x23 differ from glitch2 in all six variants. These are good candidates for the main sound parameters.

### Tail (0x8b-0xff)
- 3 files are all zero (slots 1, 2, 25).
- 11 files are zero except for a small block. 10 of them (slots 18, 19, 26, 28-32, 37, 38) have the same 12 bytes at 0xad-0xb8: `1f 01 12 21 21 12 23 41 12 25 f1 ff`. All 10 have seq 51-64. spacehunt (slot 3) has a different block at 0x98-0xa6: `11 21 11 13 41 11 15 61 11 17 f1 ff ff ff ff`.
- 25 files hold one of 7 variants of the same noise-like 117-byte pattern (starting `e3|e6|e7 3b|bb d1 8x`). The variants differ only by scattered bit flips, which looks like uninitialised RAM.

The tail is probably leftover buffer content and not part of the preset. The small blocks have a nibble pattern (`x1 y2 ...`) and might be a fragment of some other structure.

## Open questions
Mapping the fields to Kaleidoscope's controls (oscillator, wavefolder/hard-sync, LP/HP filter, comb, overdrive, FM and audio routers, ADSR, AD, 16-step modulator, modulation matrix) needs either the vendor's format description or controlled experiments. For the experiments: change one parameter on the device, export, diff.
- Is the record flag `0a` (pondfish) a per-record switch, or a sign of an older firmware/format version (pondfish also differs at 0x70, 0x71, 0x74)?
- How are header and pair_f related (both are non-standard in acidrave3 and punchface)?
- Does `0a` really encode `0` in names?
