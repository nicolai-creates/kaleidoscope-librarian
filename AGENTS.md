# Notes for coding agents

kpm is a Go TUI/CLI for Vadjuse Kaleidoscope preset files. See README.md for usage and analysis/FORMAT.md for the format.

## Code
- `format.go`: preset size, slot range, field `layout`, name charset. Field changes go here.
- `preset.go`: loading, saving (atomic), file renames, and the operations (`target*` = dry run, `op*` = apply).
- `info.go`: info text shared by CLI and TUI. `export.go`: `export` and `stats`. `tui.go`: tcell UI. `main.go`: CLI and version.
- Build and check: `go vet . && go build -o kpm .`

## Domain facts
- Presets are `impNNN.bin`, 256 bytes. Usable slots 1-126; 127 is the init preset (always empty on the synth).
- An empty `imp.bin` is the import marker, not a preset: the synth imports the folder and deletes it.
- Names: 11 bytes at 0x80, `01`-`09` = 1-9, `0b`-`24` = a-z, `0a` = 0 (unconfirmed).
- 0x00-0x7f is indexed by MIDI CC number (byte n = parameter with CC n, from the manual's MIDI parameter table); unstored CCs are `ff`. Labels are `PAGE-PARM` as on the display.
- There is no checksum (0x72 is PRST-SLOT, CC 114). Renames touch only 0x80-0x8a.
- Value scaling (raw byte vs. displayed value) is not confirmed yet; see the open questions in analysis/FORMAT.md. The analysis is based on the factory presets and the user manual alone.
- The user manual PDF lives in the working tree but must never be committed (ignored via `.git/info/exclude`). Don't copy its text into the repo; cite page names and CC numbers.

## Rules
- Never commit `Factory Presets/` or the device test exports `refactoring-*/` (both gitignored); test writes on a copy, never on the originals.
- After changing `layout`, regenerate the analysis files:
  `./kpm export json "Factory Presets" > analysis/presets.json`,
  `./kpm export csv "Factory Presets" > analysis/presets.csv`,
  `./kpm stats "Factory Presets" > analysis/stats.txt`,
  and update analysis/FORMAT.md.
- Bump `version` in main.go for releases.
