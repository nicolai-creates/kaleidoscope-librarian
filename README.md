# kpm - Kaleidoscope Preset Manager

A terminal tool for managing preset files of the [Vadjuse](https://vadjuse.ru) Kaleidoscope synthesizer: inspect presets, rename them, keep preset names and filenames in sync, move presets between slots.

> **Experimental.** The preset format is only partly known, and kpm writes and renames files in place. Things might break. Keep a backup of your presets before using it.

```
 kpm 0.3 · Factory Presets · 39 presets · import marker imp.bin present · view: device
 001  17153125       imp001.bin           │ file   imp003.bin
 002  l2             imp002.bin           │ slot   003
 003  spacehunt      imp003.bin           │ name   spacehunt
 004  motokick       imp004.bin           │ magic  ok
 005  neurokick      imp005.bin           │ fname  none
 006  pwmdron        imp006.bin           │ saved  to slot 11 (PRST-SLOT, 0x72)
 007  acidrave3      imp007.bin           │
 008  sineburn       imp008.bin           │ 1 Waveform
 009  ash            imp009.bin           │ ADTV  MIDL 64       MULT 127      MIX 13        DRIV 91
 010  scratch        imp010.bin           │ SUBT  FILT C2h      RESO 46       DELA 5        FEDB 92
 011  melanholia     imp011.bin           │ FREQ  TRAN 0        FINE 68       GLID 36       SCAL OFF
 012  energycore     imp012.bin           │ MAST  VOLU 127      FMVO 26       FMSR 1···     SEND 1234
 013  lostinspacy    imp013.bin           │ 2 Volume envelope
 014  unstable       imp014.bin           │ vENV  SHAP 0        PERI 36       SUST 36       RELE 61
 ...
↑↓ move  Tab/v view  J/K scroll  r rename  ? help  q quit  1-7 group
```

## Build

Requires Go 1.27 or newer.

```sh
go build -o kpm .
```

## Preset files

- One preset per file, `impNNN.bin`, 256 bytes. `NNN` is the slot, 001-126. Slot 127 is the init preset and always loads as an empty preset on the synth.
- The preset name is stored inside the file: up to 11 characters, lowercase `a`-`z` and digits.
- An empty `imp.bin` in the folder is the import marker: it tells the synth to import the presets in that folder. The synth deletes it after the import.

kpm can add the preset name to the filename (`imp003.bin` → `imp003_spacehunt.bin`) to make a folder easier to browse. The synth probably expects plain `impNNN.bin` names, so strip the names again (`S` in the TUI, `kpm strip`) before handing the folder back to the synth.

## Usage

```
kpm [dir]                         interactive TUI
kpm list [dir]                    one line per preset
kpm info FILE...                  fields and hex dump
kpm rename FILE NAME              set the stored preset name
kpm name2file [-n] FILE|DIR...    imp003.bin -> imp003_spacehunt.bin
kpm file2name [-n] FILE|DIR...    imp003_foo.bin -> stored name "foo"
kpm strip [-n] FILE|DIR...        imp003_spacehunt.bin -> imp003.bin
kpm export json|csv [dir]         decoded dump of all presets to stdout
kpm stats [dir]                   per-byte statistics across all presets
kpm version                       print the version
```

`-n` is a dry run. Without a directory, kpm uses the current directory, or `Factory Presets` if the current directory holds no `.bin` files.

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
| `v` / Tab | switch view: parameters, device, hex dump |
| `d` | parameters view: decimal values on/off (on by default) |
| `1`-`7` | jump to a page group (parameters and device view) |
| `0` | parameters view: jump to the bytes outside any page |
| mouse wheel | scroll the pane under the pointer |
| `?` / `h` | show all keys |
| `J` / `K` | scroll the info pane |
| `g` | reload the folder |
| `q` / Esc | quit |

The **parameters** view lists every stored value with its byte offset, ordered like the synth: page group, page, encoder. The **device** view shows one line per page and one column per encoder, with values converted to what the synth displays (e.g. `C2h`, `tr2`, `-128`, the step sequence as bars). Values whose encoding is not known yet are shown dimmed as raw numbers. The **hex dump** colours the bytes by field.

Actions on all presets ask for confirmation first. In the list, yellow marks a preset whose filename name differs from the stored name, whose slot is outside 1-126, or whose magic bytes are wrong.

## Format

What is known about the file format is in [analysis/FORMAT.md](analysis/FORMAT.md). It is based on the 39 factory presets, the user manual and a few test exports from a synth running firmware v81. Every parameter byte is mapped to its parameter, but only part of the value encodings is confirmed: time parameters such as glide, envelope period and release are not decoded yet. See [Limits of this analysis](analysis/FORMAT.md#limits-of-this-analysis) before relying on it.

`analysis/presets.json`, `presets.csv` and `stats.txt` are generated by `kpm export` and `kpm stats`.

## License

MIT, see [LICENSE](LICENSE).
