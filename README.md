# Kaleidoscope Librarian (kalib)

A terminal tool for managing preset files of the [Vadjuse](https://vadjuse.ru) Kaleidoscope synthesizer: inspect presets, rename them, keep preset names and filenames in sync, move presets between slots.

**Vision:** kalib is a librarian for the Kaleidoscope: it organizes preset files on your computer and moves presets and firmware to and from the synth through its USB drive. (Firmware transfer is not there yet.)

> **Experimental.** The preset format is only partly known, and kalib writes and renames files in place. Things might break. Keep a backup of your presets before using it.

```
 kalib 0.4 · Factory Presets · 39 presets · import marker imp.bin present · view: device
 001  06042014       imp001.bin           │ name   spacehunt    ·  imp003.bin
 002  l1             imp002.bin           │ slot   003          ·  saved to 11
 003  spacehunt      imp003.bin           │
 004  motokick       imp004.bin           │ 1 Waveform
 005  neurokick      imp005.bin           │ ADTV  MIDL 64       MULT 127      MIX  13       DRIV 91
 006  pwmdron        imp006.bin           │ SUBT  FILT C2h      RESO 46       DELA 5        FEDB 92
 007  acidrave2      imp007.bin           │ FREQ  TRAN 0        FINE 68       GLID 36       SCAL OFF
 008  sineburn       imp008.bin           │ MAST  VOLU 127      FMVO 26       FMSR 1···     SEND 1234
 009  ash            imp009.bin           │
 010  scratch        imp010.bin           │ 2 Volume envelope
 011  melanholia     imp011.bin           │ vENV  SHAP 0        PERI 36       SUST 36       RELE 61
 012  energycore     imp012.bin           │ vSEQ  EOC  0        LINK 1···     PATN 255      PSIZ 7
 013  lostinspacy    imp013.bin           │ vMX1  PARM FREQ     DEST tr1      DEPT 16       ALIA ·
 014  unstable       imp014.bin           │ vMX2  PARM FILT     DEST tr1      DEPT 50       ALIA ·
 ...
↑↓ move  Tab/v view  J/K scroll  r rename  ? help  q quit  1-7 group
```

## Build

Requires Go 1.27 or newer.

```sh
git clone https://github.com/nicolai-creates/kaleidoscope-librarian
cd kaleidoscope-librarian
go build -o kalib .
```

## Preset files

- One preset per file, `impNNN.bin`, 256 bytes. `NNN` is the slot, 000-127 (the range vadjuse's preset manager accepts). Slot 127 is the init preset and always loads as an empty preset on the synth; whether slot 000 is usable is not known.
- The preset name is stored inside the file: up to 11 characters, `a`-`z`, digits and spaces. In filenames kalib writes a space as `_`.
- An empty `imp.bin` in the folder is the import marker: it tells the synth to import the presets in that folder. The synth deletes it after the import.
- Exports from the synth are `expNNN.bin`, the preset of slot NNN. An empty `expNNN.bin` together with an empty `exp.bin` (export marker) asks the synth to export that slot. kalib lists exported presets like `imp` files (renaming, name in filename, move all work) and shows unanswered requests dimmed.

kalib can add the preset name to the filename (`imp003.bin` → `imp003_spacehunt.bin`) to make a folder easier to browse. The synth probably expects plain `impNNN.bin` names, so strip the names again (`S` in the TUI, `kalib strip`) before handing the folder back to the synth.

## Usage

```
kalib [dir]                         interactive TUI
kalib list [dir]                    one line per preset
kalib info FILE...                  fields and hex dump
kalib rename FILE NAME              set the stored preset name
kalib name2file [-n] FILE|DIR...    imp003.bin -> imp003_spacehunt.bin
kalib file2name [-n] FILE|DIR...    imp003_foo.bin -> stored name "foo"
kalib strip [-n] FILE|DIR...        imp003_spacehunt.bin -> imp003.bin
kalib device                        print where the synth's drive (KLRU) is mounted
kalib eject                         flush and eject the synth's drive
kalib request [-n] [DIR] SLOTS      ask the synth to export slots (5, 1-10, 3,7, all)
kalib import [-n] FILE SLOT [DIR]   copy a preset to DIR for import into SLOT
kalib fetch [-n] [TARGET [DIR]]     copy exported presets to TARGET (backup)
kalib clean [-n] [-y] [DIR]         delete export files (expNNN.bin, exp.bin, expc.bin)
kalib export json|csv [dir]         decoded dump of all presets to stdout
kalib stats [dir]                   per-byte statistics across all presets
kalib version                       print the version
```

`-n` is a dry run. Without a directory, kalib uses the current directory, or `Factory Presets` if the current directory holds no `.bin` files.

Transfers with the synth (as in vadjuse's preset manager): `kalib request /run/media/$USER/KLRU 1-10` creates export requests on the synth's drive; after the synth has exported, the presets are `exp001.bin`..`exp010.bin` there. `kalib import mypreset.bin 12 /run/media/$USER/KLRU` copies a preset to the drive as `imp012.bin` and sets the import marker. Neither command overwrites an existing file for the same slot. The preset is copied unchanged, including its saved-slot byte.

A full backup of the synth: `kalib clean` deletes old export files from the drive (`expNNN.bin`, `exp.bin`, `expc.bin`; it lists them and asks first, `-y` skips the question), `kalib request all` asks for every slot, and after the synth has exported, `kalib fetch ~/kaleidoscope/backup-2026-11-09` copies the exported presets (non-empty `expNNN.bin`) unchanged to that folder, creating it if needed. Without TARGET, fetch uses `$KALIB_FETCH_DIR`. It skips slots that already have an exp file in the target, so use a new folder per backup. Old exports matter because `request` skips slots that have an exp file; in the TUI, `e` offers to delete them first.

kalib finds the synth by its drive name `KLRU`: on Linux in the mount table (`/run/media/$USER/KLRU`, `/media/$USER/KLRU`), on macOS under `/Volumes`, on Windows by drive label. `request`, `import`, `fetch` and `clean` use it when DIR is left out (`kalib request 1-10`, `kalib import mypreset.bin 12`). `KALIB_DEVICE=/path` overrides the search.

`kalib eject` (TUI `E`) flushes and ejects the drive, so the synth sees complete files: on Linux with `udisksctl` (unmount, then power off), on macOS with `diskutil eject`, on Windows directly through the drive (flush, lock, dismount, eject). Close shells and file managers that have the drive open first.

Finding and ejecting are tested on Linux only so far; macOS and Windows are untested.

### TUI keys

| Key | Action |
|---|---|
| ↑ ↓ / j k, PgUp PgDn, Home End | move |
| `r` | rename the preset (also updates a name in the filename) |
| `n` / `N` | add the preset name to the filename (selected / all) |
| `f` / `F` | set the preset name from the filename (selected / all) |
| `s` / `S` | strip the name from the filename (selected / all) |
| `m` | move the preset to another slot |
| `i` | create or remove the `imp.bin` import marker |
| `e` | ask the synth to export slots: empty `expNNN.bin` files plus `exp.bin`; offers to delete old export files first |
| `b` | fetch exported presets (non-empty `expNNN.bin`) to a folder, e.g. a backup; suggests `$KALIB_FETCH_DIR` |
| `X` | delete export files (`expNNN.bin`, `exp.bin`, `expc.bin`) after showing them |
| `D` | open the synth's drive |
| `E` | eject the synth's drive |
| `c` | copy the preset to a folder (e.g. the synth's drive) as `impNNN.bin` for import into a slot, plus `imp.bin` |
| `v` / Tab | switch view: device, parameters, hex dump |
| `d` | parameters view: decimal values on/off (on by default) |
| `1`-`7` | jump to a page group (parameters and device view) |
| `0` | parameters view: jump to the bytes outside any page |
| mouse wheel | scroll the pane under the pointer |
| `?` / `h` | show all keys |
| `J` / `K` | scroll the info pane |
| `g` | reload the folder |
| `q` / Esc | quit |

The **device** view (the default) shows one line per page and one column per encoder, with values converted to what the synth displays (e.g. `C2h`, `tr2`, `-128`, the step sequence as bars). Values whose encoding is not known yet are shown dimmed as raw numbers. The **parameters** view lists every stored value with its byte offset, ordered like the synth: page group, page, encoder. The **hex dump** colours the bytes by field.

Actions on all presets ask for confirmation first. In the list, yellow marks a preset whose filename name differs from the stored name, whose slot is 127 (init preset), or whose magic bytes are wrong.

## Format

What is known about the file format is in [analysis/FORMAT.md](analysis/FORMAT.md). It is based on the 39 factory presets, the user manual and a few test exports from a synth running firmware v81. Every parameter byte is mapped to its parameter, but only part of the value encodings is confirmed: time parameters such as glide, envelope period and release are not decoded yet. See [Limits of this analysis](analysis/FORMAT.md#limits-of-this-analysis) before relying on it.

`analysis/presets.json`, `presets.csv` and `stats.txt` are generated by `kalib export` and `kalib stats`.

## License

MIT, see [LICENSE](LICENSE).
