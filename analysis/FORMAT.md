# Kaleidoscope single-voice preset (impNNN.bin), 256 bytes

Source: `exchange/kaleidoscope-presets/Factory Presets/` (39 files, plus an empty `imp.bin`: an import marker that tells the synth to import the presets in the folder; the synth deletes it after importing).
Decoder: `scripts/kaleidoscope-presets.py`. Outputs: `presets.json` (nested), `presets.csv` (one row per preset, one column per byte).

## Confirmed
| Offset | Field | Notes |
|---|---|---|
| 0x00-0x03 | magic | `ff ff ff ff` in all 39 files |
| 0x80-0x8a (128-138) | **name** | 11 bytes, `00`-padded. Not null-terminated at 11 chars (`lostinspacy`) |

Name charset: `01`-`09` = `1`-`9`, `0A` = `0`, `0B`-`24` = `a`-`z`. Lowercase, digits only, no spaces or punctuation.
Check: all 39 names decode to readable words, and `glitch2`..`glitch8` fall in consecutive slots 26-32.

## Layout (structure verified, meaning not)
Bytes in these positions are `ff` in every file and act as separators or padding: 26, 37, 47, 51, 55, 63, 69, 75, 79, 83, 85-111, 115, 117-127.

| Group | Bytes | Observation |
|---|---|---|
| header | 4-5 | 4 distinct values each; `07 ff` in most files |
| sec_a | 6-25 | 20 bytes, high variance: probably the oscillator / filter / main sound parameters |
| sec_b | 27-36 | 10 bytes, medium variance |
| sec_c | 38-46 | 9 bytes |
| slot_d, slot_e | 48-50, 52-54 | 3-byte records; default `40 00 00` |
| slot_f, slot_g, slot_h | 56-62, 64-68, 70-74 | each ends `40 3c 3c` by default; looks like repeated modulation or mixer slots |
| slot_i, slot_j | 76-78, 80-82 | 3-byte records; default `00 00 40` |
| sec_k | 84 | `0f` in every file |
| sec_l | 112-114 | `10 04|06|14 xx`; byte 114 is unique in all 39 files (checksum, seed or id?) |
| sec_m | 116 | `04` in most files |
| tail | 139-255 | `00` in some files, noise-like in others (`e3 bb d1 80 b5` recurs); probably uninitialised memory |

The labels `sec_*` and `slot_*` are positional, not functional. Mapping them to Kaleidoscope's controls (oscillator, wavefolder/hard-sync, LP/HP filter, comb, overdrive, FM and audio routers, ADSR, AD, 16-step modulator, modulation matrix) needs either the vendor's format description or controlled experiments. For the experiments: change one parameter on the device, export, diff.
