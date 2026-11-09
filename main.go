// SPDX-License-Identifier: MIT

// kpm - Kaleidoscope Preset Manager.
//
// Manages Vadjuse Kaleidoscope single-voice presets (impNNN.bin, 256 bytes).
// See analysis/FORMAT.md for what is known about the format. The field layout
// in format.go mirrors that document; extend `layout` as more is decoded.
//
// Usage:
//
//	kpm [dir]                         interactive TUI (default)
//	kpm list [dir]                    one line per preset
//	kpm info FILE...                  fields and hex dump
//	kpm rename FILE NAME              set the preset name stored in the file
//	kpm name2file [-n] FILE|DIR...    imp003.bin -> imp003_spacehunt.bin
//	kpm file2name [-n] FILE|DIR...    imp003_foo.bin -> stored name "foo"
//	kpm strip [-n] FILE|DIR...        imp003_spacehunt.bin -> imp003.bin
//	kpm device                        print where the synth's drive (KLRU) is mounted
//	kpm eject                         flush and eject the synth's drive
//	kpm request [-n] [DIR] SLOTS      ask the synth to export slots (5, 1-10, 3,7)
//	kpm import [-n] FILE SLOT [DIR]   copy a preset to DIR for import into SLOT
//	kpm export json|csv [dir]         decoded dump of all presets to stdout
//	kpm stats [dir]                   per-byte statistics across all presets
//	kpm version                       print the version
//
// -n is a dry run. request and import use the synth's drive if DIR is
// omitted. Without a dir, the current directory is used, or
// "Factory Presets" if the current directory holds no .bin files.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// version is the kpm release; override with -ldflags "-X main.version=...".
var version = "0.4"

// ---------------------------------------------------------------------------
// CLI

func main() {
	args := os.Args[1:]
	if len(args) == 0 || !isCommand(args[0]) {
		dir := ""
		if len(args) > 0 {
			dir = args[0]
		}
		if err := runTUI(defaultDir(dir)); err != nil {
			fatal(err)
		}
		return
	}
	cmd, args := args[0], args[1:]
	var err error
	switch cmd {
	case "list":
		err = cmdList(args)
	case "info":
		err = cmdInfo(args)
	case "rename":
		err = cmdRename(args)
	case "name2file", "file2name", "strip":
		err = cmdBatch(cmd, args)
	case "device":
		var dir string
		if dir, err = findDevice(); err == nil {
			fmt.Println(dir)
		}
	case "eject":
		var dir string
		if dir, err = findDevice(); err == nil {
			var note string
			if note, err = ejectDevice(dir); err == nil {
				fmt.Println("ejected", dir+": safe to unplug")
				if note != "" {
					fmt.Println("note:", note)
				}
			}
		}
	case "request":
		err = cmdRequest(args)
	case "import":
		err = cmdImport(args)
	case "export":
		err = cmdExport(args)
	case "stats":
		err = cmdStats(args)
	case "version", "--version":
		fmt.Println("kpm", version)
	case "help", "-h", "--help":
		usage()
	}
	if err != nil {
		fatal(err)
	}
}

func isCommand(s string) bool {
	switch s {
	case "list", "info", "rename", "name2file", "file2name", "strip", "device", "eject", "request", "import", "export", "stats", "version", "--version", "help", "-h", "--help":
		return true
	}
	return false
}

// warning is shown at the bottom of the CLI help and as a popup when the TUI starts.
var warning = []string{
	"WARNING: kpm is experimental. The preset format is only partly known,",
	"and kpm writes and renames files in place. Things might break.",
	"Keep a backup of your presets before using it.",
}

func usage() {
	fmt.Fprintf(os.Stderr, "kpm %s - Kaleidoscope Preset Manager\n\n", version)
	fmt.Fprint(os.Stderr, `usage:
  kpm [dir]                         interactive TUI
  kpm list [dir]                    one line per preset
  kpm info FILE...                  fields and hex dump
  kpm rename FILE NAME              set the stored preset name
  kpm name2file [-n] FILE|DIR...    imp003.bin -> imp003_spacehunt.bin
  kpm file2name [-n] FILE|DIR...    imp003_foo.bin -> stored name "foo"
  kpm strip [-n] FILE|DIR...        imp003_spacehunt.bin -> imp003.bin
  kpm device                        print where the synth's drive (KLRU) is mounted
  kpm eject                         flush and eject the synth's drive
  kpm request [-n] [DIR] SLOTS      ask the synth to export slots (5, 1-10, 3,7)
  kpm import [-n] FILE SLOT [DIR]   copy a preset to DIR for import into SLOT
  kpm export json|csv [dir]         decoded dump of all presets to stdout
  kpm stats [dir]                   per-byte statistics across all presets
  kpm version                       print the version
  -n: dry run; DIR defaults to the synth's drive

`)
	fmt.Fprintln(os.Stderr, strings.Join(warning, "\n"))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "kpm:", err)
	os.Exit(1)
}

func defaultDir(dir string) string {
	if dir != "" {
		return dir
	}
	if m, _ := filepath.Glob("*.bin"); len(m) == 0 {
		if st, err := os.Stat("Factory Presets"); err == nil && st.IsDir() {
			return "Factory Presets"
		}
	}
	return "."
}

// expandArgs turns files and directories into a preset list.
func expandArgs(args []string) ([]*Preset, error) {
	if len(args) == 0 {
		args = []string{defaultDir("")}
	}
	var ps []*Preset
	for _, a := range args {
		st, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if st.IsDir() {
			d, err := scanDir(a)
			if err != nil {
				return nil, err
			}
			ps = append(ps, d...)
		} else {
			ps = append(ps, loadPreset(a))
		}
	}
	return ps, nil
}

func listLine(p *Preset) string {
	slot := "---"
	if s, _, ok := p.Slot(); ok {
		slot = fmt.Sprintf("%03d", s)
	}
	if !p.Valid() {
		return fmt.Sprintf("%s  %-11s    %s  (%v)", slot, "", p.File(), p.Err)
	}
	st := p.SuffixState()
	if st == "" {
		st = " "
	}
	return fmt.Sprintf("%s  %-11s %s  %s", slot, p.Name(), st, p.File())
}

func cmdList(args []string) error {
	ps, err := expandArgs(args)
	for _, p := range ps {
		fmt.Println(listLine(p))
	}
	for _, a := range args {
		if st, e := os.Stat(a); e == nil && st.IsDir() {
			for _, m := range markers(a) {
				fmt.Printf("(%s: %s present)\n", a, m)
			}
		}
	}
	if len(args) == 0 {
		for _, m := range markers(defaultDir("")) {
			fmt.Printf("(%s present)\n", m)
		}
	}
	return err
}

func cmdInfo(args []string) error {
	ps, err := expandArgs(args)
	if err != nil {
		return err
	}
	for i, p := range ps {
		if i > 0 {
			fmt.Println()
		}
		for _, l := range summaryLines(p) {
			fmt.Println(l)
		}
		if !p.Valid() {
			continue
		}
		fmt.Println()
		for _, b := range fieldBlocks(p, false, 16) {
			for _, l := range b {
				fmt.Println(l)
			}
		}
		fmt.Println()
		for o := 0; o < presetSize; o += 16 {
			fmt.Printf("%02x  %s\n", o, hexBytes(p.Data[o:o+16]))
		}
	}
	return nil
}

func cmdRename(args []string) error {
	if len(args) != 2 {
		usage()
		return errors.New("rename needs FILE and NAME")
	}
	p := loadPreset(args[0])
	old := p.Name()
	if err := opRename(p, args[1]); err != nil {
		return err
	}
	fmt.Printf("%s: %q -> %q\n", p.File(), old, p.Name())
	return nil
}

// dirOrDevice returns the folder given on the command line, or the synth's
// drive if there is none.
func dirOrDevice(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return findDevice()
}

// cmdRequest creates empty expNNN.bin files and the exp.bin marker: the
// synth fills them with the presets of those slots.
func cmdRequest(args []string) error {
	fs := flag.NewFlagSet("request", flag.ExitOnError)
	dry := fs.Bool("n", false, "dry run")
	fs.Parse(args)
	if fs.NArg() < 1 || fs.NArg() > 2 {
		return errors.New("usage: kpm request [-n] [DIR] SLOTS")
	}
	dir, err := dirOrDevice(fs.Args()[:fs.NArg()-1])
	if err != nil {
		return err
	}
	slots, err := parseSlots(fs.Arg(fs.NArg() - 1))
	if err != nil {
		return err
	}
	create, skipped, err := targetRequests(dir, slots)
	if err != nil {
		return err
	}
	for _, f := range skipped {
		fmt.Printf("%s exists, skipped\n", f)
	}
	if len(create) == 0 {
		return nil
	}
	if !*dry {
		if err := opRequests(dir, create); err != nil {
			return err
		}
	}
	fmt.Printf("export requests: %s (marker %s)\n", strings.Join(create, " "), expMarkerFile)
	return nil
}

// cmdImport copies a preset to DIR as impNNN.bin and sets the imp.bin
// marker, so the synth loads it into SLOT.
func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	dry := fs.Bool("n", false, "dry run")
	fs.Parse(args)
	if fs.NArg() < 2 || fs.NArg() > 3 {
		return errors.New("usage: kpm import [-n] FILE SLOT [DIR]")
	}
	p := loadPreset(fs.Arg(0))
	slot, err := strconv.Atoi(fs.Arg(1))
	if err != nil {
		return fmt.Errorf("not a slot: %q", fs.Arg(1))
	}
	dir, err := dirOrDevice(fs.Args()[2:])
	if err != nil {
		return err
	}
	path, err := targetImport(p, dir, slot)
	if err != nil {
		return err
	}
	if !*dry {
		if _, err := opImport(p, dir, slot); err != nil {
			return err
		}
	}
	fmt.Printf("%s -> %s (marker %s)\n", p.File(), path, markerFile)
	return nil
}

func cmdBatch(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dry := fs.Bool("n", false, "dry run")
	fs.Parse(args)
	ps, err := expandArgs(fs.Args())
	if err != nil {
		return err
	}
	failed := 0
	for _, p := range ps {
		if !p.Valid() {
			continue
		}
		old, oldName := p.File(), p.Name()
		var target string
		var err error
		switch cmd {
		case "name2file":
			target, err = targetNameToFile(p)
		case "strip":
			target, err = targetStrip(p)
		case "file2name":
			target, err = targetFileToName(p)
		}
		if err == nil && !*dry {
			switch cmd {
			case "name2file":
				err = opNameToFile(p)
			case "strip":
				err = opStrip(p)
			case "file2name":
				err = opFileToName(p)
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "kpm:", err)
			failed++
			continue
		}
		if cmd == "file2name" {
			if oldName != target {
				fmt.Printf("%s: name %q -> %q\n", old, oldName, target)
			}
		} else if old != target {
			fmt.Printf("%s -> %s\n", old, target)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d file(s) failed", failed)
	}
	return nil
}
