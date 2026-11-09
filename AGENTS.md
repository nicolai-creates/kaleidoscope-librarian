# Notes for coding agents

kpm is a Go TUI/CLI for Vadjuse Kaleidoscope preset files. See README.md for usage and analysis/FORMAT.md for the format.

## Code
- `format.go`: preset size, slot range, field `layout`, name charset. Field changes go here.
- `preset.go`: loading, saving (atomic), file renames, and the operations (`target*` = dry run, `op*` = apply).
- `device.go` + `device_<os>.go`: finding (`findVolume`) and ejecting (`ejectVolume`) the synth's drive `KLRU` per platform; only Linux is tested. Check other platforms with `GOOS=darwin|windows go vet .`.
- `info.go`: info text shared by CLI and TUI. `export.go`: `export` and `stats`. `tui.go`: tcell UI. `main.go`: CLI and version.
- Build and check: `go vet . && go build -o kpm .`

## Domain facts
- Presets are `impNNN.bin`, 256 bytes. Slots 0-127 as in vadjuse's manager; 127 is the init preset (always empty on the synth), slot 0 is untested.
- An empty `imp.bin` is the import marker, not a preset: the synth imports the folder and deletes it.
- `expNNN.bin` is a preset exported by the synth (same format); empty, it is an export request (with the marker `exp.bin`). Filename operations keep the `imp`/`exp` prefix.
- Names: 11 bytes at 0x80, `00` = blank (trailing = padding), `01`-`0a` = 0-9, `0b`-`24` = a-z (from vadjuse's manager, confirmed by device exports). `_` in a filename stands for a blank.
- 0x00-0x7f is indexed by MIDI CC number (byte n = parameter with CC n, from the manual's MIDI parameter table); unstored CCs are `ff`. Labels are `PAGE-PARM` as on the display.
- There is no checksum (0x72 is PRST-SLOT, CC 114). Renames touch only 0x80-0x8a.
- Value scaling (raw byte vs. displayed value) is not confirmed yet; see the open questions in analysis/FORMAT.md. The analysis is based on the factory presets, the user manual, device exports and vadjuse's preset manager.
- The user manual PDF lives in the working tree but must never be committed (ignored via `.git/info/exclude`). Don't copy its text into the repo; cite page names and CC numbers.

## Rules
- Never commit `Factory Presets/` or the device test exports `refactoring-*/` (both gitignored); test writes on a copy, never on the originals.
- After changing `layout`, regenerate the analysis files:
  `./kpm export json "Factory Presets" > analysis/presets.json`,
  `./kpm export csv "Factory Presets" > analysis/presets.csv`,
  `./kpm stats "Factory Presets" > analysis/stats.txt`,
  and update analysis/FORMAT.md.
- For releases: bump `version` in main.go (and the sample screen in README.md), add an entry to RELEASE.md, tag `vX.Y.Z`.
