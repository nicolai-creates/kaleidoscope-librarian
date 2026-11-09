# Releases

Newest first. Each release is tagged `vX.Y.Z`. kpm is experimental: before 1.0, a minor version (0.x) may change output formats.

## Unreleased

- Export requests: `kpm request DIR SLOTS` (TUI `e`) creates empty `expNNN.bin` files for single slots, ranges or lists (`5`, `1-10`, `3,7`) plus the `exp.bin` marker. Slots with an existing exp file are skipped.

## 0.4 (2026-11-09)

- **Names:** digits were decoded one too high. The charset is now `00` = blank, `01`-`0a` = `0`-`9`, `0b`-`24` = `a`-`z`, taken from vadjuse's preset manager (MIT) and confirmed by device exports. Factory names change: `17153125` → `06042014`, `l2` → `l1`, `acidrave3` → `acidrave2`, `glitch2`..`glitch8` → `glitch1`..`glitch7`. Filenames written by `name2file` with digits in the name now show as out of sync; run `n`/`N` (or `kpm name2file`) again.
- Names may contain spaces (`space 3`). A space is `_` in filenames: `imp100_space_3.bin`.
- TUI: header white on dark blue, status line dark blue on orange.
- Exported presets (`expNNN.bin`) are recognised like `impNNN.bin`; renames and moves keep the prefix. Empty `expNNN.bin` files show as export requests, and `exp.bin` as the export marker, instead of size errors.
- A firmware file `blink.bin` in the folder is no longer listed as a broken preset.
- Info header: two lines instead of five (`name · file`, `slot · saved to`), with name and slot padded so the columns stay put. The filename state is only mentioned when it differs from the name.
- Slots 0-127 as in vadjuse's preset manager (was 1-126). Slot 127 is still marked as the init preset, which loads empty.

## 0.3.2 (2026-10-30)

- Parameters view: sSEQ › VALU gets a third row with the step values as the synth shows them (-36..36). All three rows (hex, decimal, value) are four columns per step and line up.
- Presets saved before firmware v76 show that row dimmed: those bytes are leftover data, not steps.

## 0.3.1 (2026-10-30)

- The device view is now the first view and the one kpm opens in. Views cycle device, parameters, hex dump.
- Device view: encoder labels are padded to four characters, so values line up in columns.
- Header: the magic bytes are only shown when they are wrong (`magic  BAD …`, red).

## 0.3 (2026-10-30)

TUI rework.

- New **device** view: one row per synth page, one column per encoder, values converted to what the synth displays where the encoding is known (`C2h`, `tr2`, `FILT`, `mn1`, depth -128..128, track masks like `1·3·`). Unknown encodings are shown dimmed as raw numbers. The step sequence is drawn as bars.
- **Parameters** view (was "fields hex" / "fields dec"): ordered like the synth (page group, page, encoder) instead of by byte offset, decimal values on by default (`d` toggles). Bytes outside any page are listed under "0 File".
- Keys `1`-`7` jump to a page group, `0` to the File section. `?` or `h` opens the full key list; the status line shows only the main keys plus those of the current view.
- The mouse wheel scrolls the pane under the pointer: the preset list or the info pane. Text selection in the terminal now needs Shift.
- Hex dump: step values in their own colour.
- Header: the raw name bytes are gone (they are in the parameters view and the hex dump); `saved  to slot N` replaces `slot?`.
- Presets saved before firmware v76 show the step modulator as "not stored".
- Labels follow the v81 display: `bMX` (pitch bend, pMX in the manual), `vCHN`, `mCHN`, `pCHN`.

## 0.2 (2026-10-30)

Parameter names.

- Every byte of the parameter area is mapped to its synth parameter: the byte at offset n holds the parameter with MIDI CC n. Fields are named `PAGE-PARM` as on the display with firmware v81 (e.g. `ADTV-MULT`, `SUBT-FILT`), replacing the positional labels (`sec_a`, `rec_f`, …).
- Value encodings confirmed with test exports from a v81 synth: track numbers and masks, TSEL, BPM, TRAN, DEPT, FILT (bit 7 = high-pass), the MIDI channels, the step modulator and the PARM list. 0x72 is the slot the preset was saved to (PRST-SLOT), not a checksum.
- analysis/FORMAT.md rewritten around the CC map, with a section on the limits of the analysis (glide and the other time parameters are not decoded).
- Output changes: `kpm info` prints one line per parameter; the JSON export renames `seq` to `saved_slot`.

## 0.1 (2026-10-26, not tagged)

First version.

- TUI and CLI for a folder of `impNNN.bin` presets: list, info, rename, move between slots (1-126), import marker `imp.bin`.
- Keep preset names and filenames in sync: `name2file`, `file2name`, `strip`.
- `export json|csv` and `stats` for analysing the format; fields labelled by position only.
