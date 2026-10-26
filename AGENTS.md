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
- 0x72 is a save sequence number, not a checksum. Renames touch only 0x80-0x8a.
- Labels like `sec_a`, `rec_f` are positional. Vadjuse will provide the official format description; when it arrives, replace them with real names.

## Rules
- Never commit `Factory Presets/` (gitignored); test writes on a copy, never on the originals.
- After changing `layout`, regenerate the analysis files:
  `./kpm export json "Factory Presets" > analysis/presets.json`,
  `./kpm export csv "Factory Presets" > analysis/presets.csv`,
  `./kpm stats "Factory Presets" > analysis/stats.txt`,
  and update analysis/FORMAT.md.
- Bump `version` in main.go for releases.
