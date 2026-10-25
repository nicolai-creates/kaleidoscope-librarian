// kpm - Kaleidoscope preset manager.
//
// Manages Vadjuse Kaleidoscope single-voice presets (impNNN.bin, 256 bytes).
// See analysis/FORMAT.md for what is known about the format. The field layout below
// mirrors that document; extend `layout` as more of the format is decoded.
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
//	kpm export json|csv [dir]         decoded dump of all presets to stdout
//	kpm stats [dir]                   per-byte statistics across all presets
//
// -n is a dry run. Without a dir, the current directory is used, or
// "Factory Presets" if the current directory holds no .bin files.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ---------------------------------------------------------------------------
// Format

const (
	presetSize = 256
	nameOff    = 0x80
	nameLen    = 11
)

var magic = []byte{0xff, 0xff, 0xff, 0xff}

// field is a byte range of the preset. Labels are positional until the
// meaning of a range is known (see analysis/FORMAT.md).
type field struct {
	Off, Len int
	Label    string
	Note     string
}

// Record types (see FORMAT.md):
//
//	A: 3 bytes (value ~40, 00-16, flag 00|0a)
//	B: 5 bytes (sel 00|1f|.., flag 00|0a, 40|bf, x, y); i and j hold only the first 3
var layout = []field{
	{0x00, 4, "magic", "ff ff ff ff"},
	{0x04, 2, "header", "07 ff in 35 files"},
	{0x06, 20, "sec_a", "high variance: osc / filter / main?"},
	{0x1b, 2, "sec_b", ""},
	{0x1d, 3, "rec_b", "record A"},
	{0x20, 5, "sec_b2", ""},
	{0x26, 6, "sec_c", "0x2a-0x2b always 04"},
	{0x2c, 3, "rec_c", "record A"},
	{0x30, 3, "rec_d", "record A"},
	{0x34, 3, "rec_e", "record A"},
	{0x38, 2, "pair_f", "ff 07 in 37 files"},
	{0x3a, 5, "rec_f", "record B; default 00 00 40 3c 3c"},
	{0x40, 5, "rec_g", "record B; default 00 00 40 3c 3c"},
	{0x46, 5, "rec_h", "record B; default 00 00 40 3c 3c"},
	{0x4c, 3, "rec_i", "record B (short); default 00 00 40"},
	{0x50, 3, "rec_j", "record B (short); default 00 00 40"},
	{0x54, 1, "sec_k", "0f in every file"},
	{0x70, 2, "sec_l", "10 04 in most files"},
	{0x72, 1, "seq", "save sequence number? unique per preset"},
	{0x74, 1, "sec_m", "04 in most files"},
	{nameOff, nameLen, "name", "1-9,a-z (0 assumed); 00-padded"},
}

// fieldAt maps each byte offset to its index in layout, or -1.
var fieldAt = func() (m [presetSize]int) {
	for i := range m {
		m[i] = -1
	}
	for i, f := range layout {
		for o := f.Off; o < f.Off+f.Len; o++ {
			if m[o] >= 0 {
				panic(fmt.Sprintf("layout: %s overlaps %s at 0x%02x", f.Label, layout[m[o]].Label, o))
			}
			m[o] = i
		}
	}
	return
}()

// Name charset: 01-09 = '1'-'9', 0a = '0', 0b-24 = 'a'-'z', 00 = padding.
func codeToChar(c byte) (rune, bool) {
	switch {
	case c >= 0x01 && c <= 0x09:
		return rune('0' + c), true
	case c == 0x0a:
		return '0', true
	case c >= 0x0b && c <= 0x24:
		return rune('a' + c - 0x0b), true
	}
	return 0, false
}

func charToCode(r rune) (byte, bool) {
	switch {
	case r >= '1' && r <= '9':
		return byte(r - '0'), true
	case r == '0':
		return 0x0a, true
	case r >= 'a' && r <= 'z':
		return byte(r-'a') + 0x0b, true
	}
	return 0, false
}

func decodeName(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c == 0 {
			break
		}
		if r, ok := codeToChar(c); ok {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('?')
		}
	}
	return sb.String()
}

func encodeName(s string) ([]byte, error) {
	s = strings.ToLower(s)
	if s == "" {
		return nil, errors.New("name is empty")
	}
	if len(s) > nameLen {
		return nil, fmt.Errorf("name %q longer than %d characters", s, nameLen)
	}
	out := make([]byte, nameLen)
	for i, r := range s {
		c, ok := charToCode(r)
		if !ok {
			return nil, fmt.Errorf("invalid character %q in name (allowed: a-z, 0-9)", r)
		}
		out[i] = c
	}
	return out, nil
}

// sanitizeName turns arbitrary text (e.g. a filename suffix) into a valid
// preset name: lowercase, invalid characters dropped, truncated.
func sanitizeName(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if _, ok := charToCode(r); ok && sb.Len() < nameLen {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Preset files

// fileRe matches impNNN.bin and impNNN_<name>.bin (also '-', ' ' or '.' as separator).
var fileRe = regexp.MustCompile(`(?i)^imp(\d{3})(?:[_ .-](.*))?\.bin$`)

type Preset struct {
	Path string
	Data []byte
	Err  error // set if the file is unreadable or not a preset
}

func loadPreset(path string) *Preset {
	p := &Preset{Path: path}
	p.Data, p.Err = os.ReadFile(path)
	if p.Err == nil && len(p.Data) != presetSize {
		p.Err = fmt.Errorf("size %d, want %d", len(p.Data), presetSize)
	}
	return p
}

func (p *Preset) File() string { return filepath.Base(p.Path) }
func (p *Preset) Valid() bool  { return p.Err == nil }

func (p *Preset) Name() string {
	if !p.Valid() {
		return ""
	}
	return decodeName(p.Data[nameOff : nameOff+nameLen])
}

func (p *Preset) MagicOK() bool {
	return p.Valid() && string(p.Data[:4]) == string(magic)
}

// Slot returns the slot number and name suffix from the filename.
func (p *Preset) Slot() (slot int, suffix string, ok bool) {
	m := fileRe.FindStringSubmatch(p.File())
	if m == nil {
		return -1, "", false
	}
	slot, _ = strconv.Atoi(m[1])
	return slot, m[2], true
}

// SuffixState is "" (no name in filename), "=" (matches) or "!" (differs).
func (p *Preset) SuffixState() string {
	_, suf, ok := p.Slot()
	switch {
	case !ok || suf == "":
		return ""
	case sanitizeName(suf) == p.Name():
		return "="
	}
	return "!"
}

func (p *Preset) save() error {
	return writeAtomic(p.Path, p.Data)
}

func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kpm-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (p *Preset) renameFile(base string) error {
	if base == p.File() {
		return nil
	}
	target := filepath.Join(filepath.Dir(p.Path), base)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists", base)
	}
	if err := os.Rename(p.Path, target); err != nil {
		return err
	}
	p.Path = target
	return nil
}

// markerFile is the empty file that tells the synth to import the presets
// in its folder. The synth deletes it after the import.
const markerFile = "imp.bin"

func isMarker(path string) bool { return strings.EqualFold(filepath.Base(path), markerFile) }

func hasMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, markerFile))
	return err == nil
}

// toggleMarker creates the import marker, or removes it if present and empty.
func toggleMarker(dir string) (created bool, err error) {
	path := filepath.Join(dir, markerFile)
	st, err := os.Stat(path)
	if err != nil {
		return true, os.WriteFile(path, nil, 0o644)
	}
	if st.Size() != 0 {
		return false, fmt.Errorf("%s is not empty, not removing it", markerFile)
	}
	return false, os.Remove(path)
}

func scanDir(dir string) ([]*Preset, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.bin"))
	if err != nil {
		return nil, err
	}
	var ps []*Preset
	for _, path := range paths {
		if !isMarker(path) {
			ps = append(ps, loadPreset(path))
		}
	}
	sortPresets(ps)
	return ps, nil
}

func sortPresets(ps []*Preset) {
	sort.SliceStable(ps, func(i, j int) bool {
		si, _, oki := ps[i].Slot()
		sj, _, okj := ps[j].Slot()
		if oki != okj {
			return oki
		}
		if si != sj {
			return si < sj
		}
		return ps[i].File() < ps[j].File()
	})
}

// ---------------------------------------------------------------------------
// Operations. target* compute the result without touching disk (dry run),
// op* apply it.

func targetNameToFile(p *Preset) (string, error) {
	slot, _, ok := p.Slot()
	if !ok {
		return "", fmt.Errorf("%s: filename is not impNNN[_name].bin", p.File())
	}
	if p.Name() == "" {
		return "", fmt.Errorf("%s: preset has no name", p.File())
	}
	return fmt.Sprintf("imp%03d_%s.bin", slot, p.Name()), nil
}

func targetStrip(p *Preset) (string, error) {
	slot, _, ok := p.Slot()
	if !ok {
		return "", fmt.Errorf("%s: filename is not impNNN[_name].bin", p.File())
	}
	return fmt.Sprintf("imp%03d.bin", slot), nil
}

func targetFileToName(p *Preset) (string, error) {
	_, suf, ok := p.Slot()
	if !ok || suf == "" {
		return "", fmt.Errorf("%s: filename carries no name", p.File())
	}
	name := sanitizeName(suf)
	if name == "" {
		return "", fmt.Errorf("%s: no valid name characters in %q", p.File(), suf)
	}
	return name, nil
}

func checkValid(p *Preset) error {
	if !p.Valid() {
		return fmt.Errorf("%s: %v", p.File(), p.Err)
	}
	return nil
}

// opRename sets the stored name. If the filename carries a name, it is
// updated too so both stay in sync.
func opRename(p *Preset, name string) error {
	if err := checkValid(p); err != nil {
		return err
	}
	enc, err := encodeName(name)
	if err != nil {
		return err
	}
	old := append([]byte(nil), p.Data[nameOff:nameOff+nameLen]...)
	copy(p.Data[nameOff:], enc)
	if err := p.save(); err != nil {
		copy(p.Data[nameOff:], old)
		return err
	}
	if _, suf, ok := p.Slot(); ok && suf != "" {
		return opNameToFile(p)
	}
	return nil
}

func opNameToFile(p *Preset) error {
	if err := checkValid(p); err != nil {
		return err
	}
	base, err := targetNameToFile(p)
	if err != nil {
		return err
	}
	return p.renameFile(base)
}

func opStrip(p *Preset) error {
	base, err := targetStrip(p)
	if err != nil {
		return err
	}
	return p.renameFile(base)
}

func opFileToName(p *Preset) error {
	if err := checkValid(p); err != nil {
		return err
	}
	name, err := targetFileToName(p)
	if err != nil {
		return err
	}
	enc, _ := encodeName(name)
	copy(p.Data[nameOff:], enc)
	return p.save()
}

// opMove changes the slot number in the filename, refusing if another
// preset in the set already uses the slot.
func opMove(p *Preset, slot int, all []*Preset) error {
	if slot < 0 || slot > 999 {
		return fmt.Errorf("slot %d out of range", slot)
	}
	cur, suf, ok := p.Slot()
	if !ok {
		return fmt.Errorf("%s: filename is not impNNN[_name].bin", p.File())
	}
	if cur == slot {
		return nil
	}
	for _, o := range all {
		if s, _, ok := o.Slot(); ok && s == slot && o != p {
			return fmt.Errorf("slot %03d is used by %s", slot, o.File())
		}
	}
	base := fmt.Sprintf("imp%03d.bin", slot)
	if suf != "" {
		base = fmt.Sprintf("imp%03d_%s.bin", slot, suf)
	}
	return p.renameFile(base)
}

// ---------------------------------------------------------------------------
// Info rendering (shared by CLI and TUI)

func summaryLines(p *Preset) []string {
	l := []string{"file   " + p.File()}
	if !p.Valid() {
		return append(l, "error  "+p.Err.Error())
	}
	slot := "-"
	if s, _, ok := p.Slot(); ok {
		slot = fmt.Sprintf("%03d", s)
	}
	nameRaw := hexBytes(p.Data[nameOff : nameOff+nameLen])
	magicState := "ok"
	if !p.MagicOK() {
		magicState = "BAD " + hexBytes(p.Data[:4])
	}
	sufState := map[string]string{"": "none", "=": "matches name", "!": "differs from name"}[p.SuffixState()]
	l = append(l,
		"slot   "+slot,
		"name   "+p.Name(),
		"raw    "+nameRaw,
		"magic  "+magicState,
		"fname  "+sufState,
		fmt.Sprintf("seq?   0x72 = %02x (%d)", p.Data[0x72], p.Data[0x72]),
	)
	return l
}

func hexBytes(b []byte) string {
	s := make([]string, len(b))
	for i, c := range b {
		s[i] = fmt.Sprintf("%02x", c)
	}
	return strings.Join(s, " ")
}

func decBytes(b []byte) string {
	s := make([]string, len(b))
	for i, c := range b {
		s[i] = fmt.Sprintf("%3d", c)
	}
	return strings.Join(s, " ")
}

// fieldBlocks returns, per layout field, a header line and value lines with
// perRow bytes each.
func fieldBlocks(p *Preset, dec bool, perRow int) [][]string {
	var out [][]string
	for _, f := range layout {
		hdr := fmt.Sprintf("%-7s 0x%02x-0x%02x  %s", f.Label, f.Off, f.Off+f.Len-1, f.Note)
		if f.Len == 1 {
			hdr = fmt.Sprintf("%-7s 0x%02x       %s", f.Label, f.Off, f.Note)
		}
		block := []string{hdr}
		b := p.Data[f.Off : f.Off+f.Len]
		for i := 0; i < len(b); i += perRow {
			row := b[i:min(i+perRow, len(b))]
			if dec {
				block = append(block, "        "+decBytes(row))
			} else {
				block = append(block, "        "+hexBytes(row))
			}
		}
		if f.Label == "name" {
			block[len(block)-1] += "   \"" + p.Name() + "\""
		}
		out = append(out, block)
	}
	return out
}

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
	case "export":
		err = cmdExport(args)
	case "stats":
		err = cmdStats(args)
	case "help", "-h", "--help":
		usage()
	}
	if err != nil {
		fatal(err)
	}
}

func isCommand(s string) bool {
	switch s {
	case "list", "info", "rename", "name2file", "file2name", "strip", "export", "stats", "help", "-h", "--help":
		return true
	}
	return false
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  kpm [dir]                         interactive TUI
  kpm list [dir]                    one line per preset
  kpm info FILE...                  fields and hex dump
  kpm rename FILE NAME              set the stored preset name
  kpm name2file [-n] FILE|DIR...    imp003.bin -> imp003_spacehunt.bin
  kpm file2name [-n] FILE|DIR...    imp003_foo.bin -> stored name "foo"
  kpm strip [-n] FILE|DIR...        imp003_spacehunt.bin -> imp003.bin
  kpm export json|csv [dir]         decoded dump of all presets to stdout
  kpm stats [dir]                   per-byte statistics across all presets
  -n: dry run
`)
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
		if st, e := os.Stat(a); e == nil && st.IsDir() && hasMarker(a) {
			fmt.Printf("(%s: import marker %s present)\n", a, markerFile)
		}
	}
	if len(args) == 0 && hasMarker(defaultDir("")) {
		fmt.Printf("(import marker %s present)\n", markerFile)
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

// ---------------------------------------------------------------------------
// Export and statistics (regenerate analysis/presets.json, presets.csv, stats.txt)

func validPresets(args []string) ([]*Preset, error) {
	ps, err := expandArgs(args)
	var out []*Preset
	for _, p := range ps {
		if p.Valid() {
			out = append(out, p)
		}
	}
	return out, err
}

func ints(b []byte) []int {
	out := make([]int, len(b))
	for i, c := range b {
		out[i] = int(c)
	}
	return out
}

func fieldRange(f field) string {
	return fmt.Sprintf("0x%02x-0x%02x", f.Off, f.Off+f.Len-1)
}

func cmdExport(args []string) error {
	if len(args) < 1 || (args[0] != "json" && args[0] != "csv") {
		return errors.New("export needs json or csv")
	}
	ps, err := validPresets(args[1:])
	if err != nil {
		return err
	}
	if args[0] == "csv" {
		return exportCSV(ps)
	}
	return exportJSON(ps)
}

func exportJSON(ps []*Preset) error {
	type jField struct {
		Label  string `json:"label"`
		Offset string `json:"offset"`
		Note   string `json:"note,omitempty"`
		Bytes  []int  `json:"bytes,omitempty"`
	}
	type jPreset struct {
		Name      string   `json:"name"`
		File      string   `json:"file"`
		Slot      int      `json:"slot"`
		Seq       int      `json:"seq"`
		NameCodes []int    `json:"name_codes"`
		MagicOK   bool     `json:"magic_ok"`
		Fields    []jField `json:"fields"`
		Tail      []int    `json:"tail"`
	}
	doc := struct {
		Synth     string    `json:"synth"`
		Kind      string    `json:"kind"`
		Generator string    `json:"generator"`
		Layout    []jField  `json:"layout"`
		Count     int       `json:"count"`
		Presets   []jPreset `json:"presets"`
	}{Synth: "Vadjuse Kaleidoscope", Kind: "single-voice preset", Generator: "kpm export json", Count: len(ps)}
	for _, f := range layout {
		doc.Layout = append(doc.Layout, jField{Label: f.Label, Offset: fieldRange(f), Note: f.Note})
	}
	for _, p := range ps {
		slot, _, _ := p.Slot()
		jp := jPreset{
			Name: p.Name(), File: p.File(), Slot: slot, Seq: int(p.Data[0x72]),
			NameCodes: ints(bytes.TrimRight(p.Data[nameOff:nameOff+nameLen], "\x00")),
			MagicOK:   p.MagicOK(),
			Tail:      ints(p.Data[nameOff+nameLen:]),
		}
		for _, f := range layout {
			jp.Fields = append(jp.Fields, jField{Label: f.Label, Offset: fieldRange(f), Bytes: ints(p.Data[f.Off : f.Off+f.Len])})
		}
		doc.Presets = append(doc.Presets, jp)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	return enc.Encode(doc)
}

// exportCSV writes one row per preset and one column per byte of every
// field except magic and name.
func exportCSV(ps []*Preset) error {
	w := csv.NewWriter(os.Stdout)
	hdr := []string{"name", "slot", "file"}
	for _, f := range layout {
		if f.Label == "magic" || f.Label == "name" {
			continue
		}
		for o := f.Off; o < f.Off+f.Len; o++ {
			hdr = append(hdr, fmt.Sprintf("%s_%02x", f.Label, o))
		}
	}
	w.Write(hdr)
	for _, p := range ps {
		slot, _, _ := p.Slot()
		row := []string{p.Name(), strconv.Itoa(slot), p.File()}
		for _, f := range layout {
			if f.Label == "magic" || f.Label == "name" {
				continue
			}
			for o := f.Off; o < f.Off+f.Len; o++ {
				row = append(row, strconv.Itoa(int(p.Data[o])))
			}
		}
		w.Write(row)
	}
	w.Flush()
	return w.Error()
}

// cmdStats prints, for every byte up to the end of the name, how many
// distinct values it takes across the presets, plus a summary of the tail.
func cmdStats(args []string) error {
	ps, err := validPresets(args)
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		return errors.New("no presets")
	}
	fmt.Printf("# per-byte statistics over %d presets (kpm stats)\n", len(ps))
	fmt.Println("# off  field    distinct  min  max  values (count), most common first")
	for o := 0; o < nameOff+nameLen; o++ {
		counts := map[byte]int{}
		lo, hi := 255, 0
		for _, p := range ps {
			c := p.Data[o]
			counts[c]++
			lo, hi = min(lo, int(c)), max(hi, int(c))
		}
		vals := make([]byte, 0, len(counts))
		for c := range counts {
			vals = append(vals, c)
		}
		sort.Slice(vals, func(i, j int) bool {
			if counts[vals[i]] != counts[vals[j]] {
				return counts[vals[i]] > counts[vals[j]]
			}
			return vals[i] < vals[j]
		})
		var top []string
		for i, c := range vals {
			if i == 6 {
				top = append(top, "...")
				break
			}
			top = append(top, fmt.Sprintf("%02x(%d)", c, counts[c]))
		}
		label := "-"
		if fi := fieldAt[o]; fi >= 0 {
			label = layout[fi].Label
		}
		fmt.Printf("0x%02x  %-8s %8d  %3d  %3d  %s\n", o, label, len(counts), lo, hi, strings.Join(top, " "))
	}

	tails := map[string][]int{}
	var order []string
	for _, p := range ps {
		t := string(p.Data[nameOff+nameLen:])
		if _, ok := tails[t]; !ok {
			order = append(order, t)
		}
		slot, _, _ := p.Slot()
		tails[t] = append(tails[t], slot)
	}
	fmt.Printf("\n# tail 0x%02x-0xff: %d distinct contents\n", nameOff+nameLen, len(tails))
	for _, t := range order {
		b := []byte(t)
		nz := 0
		for _, c := range b {
			if c != 0 {
				nz++
			}
		}
		fmt.Printf("%3d non-zero bytes, slots %v\n", nz, tails[t])
	}
	return nil
}

// ---------------------------------------------------------------------------
// TUI

const (
	viewHex = iota
	viewDec
	viewDump
	numViews
)

var viewNames = [numViews]string{"fields hex", "fields dec", "hex dump"}

type ui struct {
	s         tcell.Screen
	dir       string
	ps        []*Preset
	cur, top  int
	view      int
	scroll    int
	status    string
	statusErr bool
}

var (
	stDefault = tcell.StyleDefault
	stHeader  = stDefault.Reverse(true)
	stSel     = stDefault.Reverse(true)
	stDim     = stDefault.Foreground(tcell.ColorGray)
	stBold    = stDefault.Bold(true)
	stErr     = stDefault.Foreground(tcell.ColorRed)
	stWarn    = stDefault.Foreground(tcell.ColorYellow)
	stOK      = stDefault.Foreground(tcell.ColorGreen)
	stName    = stDefault.Foreground(tcell.ColorGreen).Bold(true)
	stMagic   = stDefault.Foreground(tcell.ColorBlue)
	stFieldA  = stDefault.Foreground(tcell.ColorAqua)
	stFieldB  = stDefault.Foreground(tcell.ColorFuchsia)
)

const helpText = "↑↓ move  r rename  n/N name→file  f/F file→name  s/S strip  m move slot  i import marker  v view  J/K scroll  g reload  q quit   (capital = all)"

func runTUI(dir string) error {
	ps, err := scanDir(dir)
	if err != nil {
		return err
	}
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	defer s.Fini()
	u := &ui{s: s, dir: dir, ps: ps}
	if len(ps) == 0 {
		u.setErr("no .bin files in " + dir)
	}
	for {
		u.draw()
		ev, ok := s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue // resize etc.: just redraw
		}
		if !u.handleKey(ev) {
			return nil
		}
	}
}

func (u *ui) setMsg(s string) { u.status, u.statusErr = s, false }
func (u *ui) setErr(s string) { u.status, u.statusErr = s, true }

func (u *ui) selected() *Preset {
	if u.cur < 0 || u.cur >= len(u.ps) {
		return nil
	}
	return u.ps[u.cur]
}

func (u *ui) move(d int) {
	u.cur = max(0, min(len(u.ps)-1, u.cur+d))
	u.scroll = 0
}

func (u *ui) handleKey(ev *tcell.EventKey) bool {
	u.status = ""
	_, h := u.s.Size()
	page := max(1, h-3)
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return false
	case tcell.KeyUp:
		u.move(-1)
	case tcell.KeyDown:
		u.move(1)
	case tcell.KeyPgUp:
		u.move(-page)
	case tcell.KeyPgDn:
		u.move(page)
	case tcell.KeyHome:
		u.move(-len(u.ps))
	case tcell.KeyEnd:
		u.move(len(u.ps))
	case tcell.KeyTab:
		u.view, u.scroll = (u.view+1)%numViews, 0
	case tcell.KeyRune:
		return u.handleRune(ev.Rune())
	}
	return true
}

func (u *ui) handleRune(r rune) bool {
	p := u.selected()
	switch r {
	case 'q':
		return false
	case 'k':
		u.move(-1)
	case 'j':
		u.move(1)
	case 'K':
		u.scroll = max(0, u.scroll-1)
	case 'J':
		u.scroll++
	case 'v':
		u.view, u.scroll = (u.view+1)%numViews, 0
	case 'g':
		u.reload()
	case 'i':
		created, err := toggleMarker(u.dir)
		if created {
			u.result(err, "import marker "+markerFile+" created: synth will import this folder")
		} else {
			u.result(err, "import marker "+markerFile+" removed")
		}
	case 'r':
		if p == nil || !p.Valid() {
			return true
		}
		name, ok := u.prompt("new name (a-z 0-9, max 11): ", p.Name())
		if !ok || name == p.Name() {
			return true
		}
		old := p.Name()
		u.result(opRename(p, name), fmt.Sprintf("renamed %q -> %q", old, p.Name()))
	case 'n':
		u.single(p, opNameToFile, "name added to filename")
	case 'N':
		u.all("add preset name to all filenames", opNameToFile)
	case 'f':
		if p != nil && p.Valid() {
			if t, err := targetFileToName(p); err == nil && t != p.Name() &&
				!u.confirm(fmt.Sprintf("set name %q -> %q? (y/n)", p.Name(), t)) {
				return true
			}
		}
		u.single(p, opFileToName, "name taken from filename")
	case 'F':
		u.all("set stored name from filename for all files", opFileToName)
	case 's':
		u.single(p, opStrip, "filename reset to impNNN.bin")
	case 'S':
		u.all("strip names from all filenames", opStrip)
	case 'm':
		if p == nil {
			return true
		}
		in, ok := u.prompt("move to slot (0-999): ", "")
		if !ok {
			return true
		}
		n, err := strconv.Atoi(strings.TrimSpace(in))
		if err != nil {
			u.setErr("not a number: " + in)
			return true
		}
		u.result(opMove(p, n, u.ps), "moved to "+p.File())
	}
	return true
}

func (u *ui) result(err error, ok string) {
	if err != nil {
		u.setErr(err.Error())
	} else {
		u.setMsg(ok)
	}
	u.resort()
}

func (u *ui) single(p *Preset, op func(*Preset) error, ok string) {
	if p == nil {
		return
	}
	u.result(op(p), ok)
}

func (u *ui) all(what string, op func(*Preset) error) {
	if !u.confirm(what + "? (y/n)") {
		return
	}
	var errs []string
	n := 0
	for _, p := range u.ps {
		if !p.Valid() {
			continue
		}
		if err := op(p); err != nil {
			errs = append(errs, err.Error())
		} else {
			n++
		}
	}
	u.resort()
	if len(errs) > 0 {
		u.setErr(fmt.Sprintf("%d ok, %d failed: %s", n, len(errs), errs[0]))
	} else {
		u.setMsg(fmt.Sprintf("%d presets done", n))
	}
}

// resort keeps the cursor on the same preset after a slot change.
func (u *ui) resort() {
	p := u.selected()
	sortPresets(u.ps)
	for i, o := range u.ps {
		if o == p {
			u.cur = i
		}
	}
}

func (u *ui) reload() {
	var path string
	if p := u.selected(); p != nil {
		path = p.Path
	}
	ps, err := scanDir(u.dir)
	if err != nil {
		u.setErr(err.Error())
		return
	}
	u.ps, u.cur = ps, 0
	for i, p := range ps {
		if p.Path == path {
			u.cur = i
		}
	}
	u.setMsg(fmt.Sprintf("reloaded %d files", len(ps)))
}

// prompt reads a line on the bottom row. Returns false on Esc.
func (u *ui) prompt(label, initial string) (string, bool) {
	buf := []rune(initial)
	for {
		u.draw()
		w, h := u.s.Size()
		u.clearRow(h-1, stDefault)
		x := u.str(0, h-1, w, stBold, label)
		x = u.str(x, h-1, w-x, stDefault, string(buf))
		u.s.ShowCursor(x, h-1)
		u.s.Show()
		ev, ok := u.s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue
		}
		switch ev.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			u.s.HideCursor()
			return "", false
		case tcell.KeyEnter:
			u.s.HideCursor()
			return string(buf), true
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case tcell.KeyCtrlU:
			buf = buf[:0]
		case tcell.KeyRune:
			buf = append(buf, ev.Rune())
		}
	}
}

func (u *ui) confirm(q string) bool {
	for {
		u.draw()
		w, h := u.s.Size()
		u.clearRow(h-1, stDefault)
		u.str(0, h-1, w, stWarn.Bold(true), q)
		u.s.Show()
		ev, ok := u.s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue
		}
		if ev.Key() == tcell.KeyRune {
			switch ev.Rune() {
			case 'y', 'Y':
				return true
			case 'n', 'N':
				return false
			}
		}
		if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
			return false
		}
	}
}

// str draws s at (x,y), clipped to maxw cells, and returns the next x.
func (u *ui) str(x, y, maxw int, st tcell.Style, s string) int {
	for _, r := range s {
		if maxw <= 0 {
			break
		}
		u.s.SetContent(x, y, r, nil, st)
		x++
		maxw--
	}
	return x
}

func (u *ui) clearRow(y int, st tcell.Style) {
	w, _ := u.s.Size()
	for x := 0; x < w; x++ {
		u.s.SetContent(x, y, ' ', nil, st)
	}
}

const listWidth = 42

func (u *ui) draw() {
	s := u.s
	s.Clear()
	w, h := s.Size()
	if h < 4 || w < 20 {
		s.Show()
		return
	}

	u.clearRow(0, stHeader)
	marker := "no import marker"
	if hasMarker(u.dir) {
		marker = "import marker " + markerFile + " present"
	}
	u.str(1, 0, w-2, stHeader, fmt.Sprintf("kpm · %s · %d presets · %s · view: %s", u.dir, len(u.ps), marker, viewNames[u.view]))

	lw := min(listWidth, w/2)
	rows := h - 2
	if u.cur < u.top {
		u.top = u.cur
	}
	if u.cur >= u.top+rows {
		u.top = u.cur - rows + 1
	}
	for i := 0; i < rows && u.top+i < len(u.ps); i++ {
		idx := u.top + i
		p := u.ps[idx]
		st := stDefault
		switch {
		case !p.Valid():
			st = stErr
		case p.SuffixState() == "!" || !p.MagicOK():
			st = stWarn
		}
		if idx == u.cur {
			st = st.Reverse(true)
		}
		y := 1 + i
		for x := 0; x < lw; x++ {
			s.SetContent(x, y, ' ', nil, st)
		}
		u.str(1, y, lw-1, st, listLine(p))
	}
	for y := 1; y < h-1; y++ {
		s.SetContent(lw, y, '│', nil, stDim)
	}

	if p := u.selected(); p != nil {
		u.drawInfo(p, lw+2, 1, w-lw-2, h-2)
	}

	u.clearRow(h-1, stDefault)
	switch {
	case u.status == "":
		u.str(0, h-1, w, stDim, helpText)
	case u.statusErr:
		u.str(0, h-1, w, stErr.Bold(true), u.status)
	default:
		u.str(0, h-1, w, stOK, u.status)
	}
	s.Show()
}

type styledLine struct {
	segs []styledSeg
}

type styledSeg struct {
	st tcell.Style
	s  string
}

func line(st tcell.Style, s string) styledLine { return styledLine{[]styledSeg{{st, s}}} }

func (u *ui) drawInfo(p *Preset, x0, y0, w, h int) {
	var lines []styledLine
	for _, l := range summaryLines(p) {
		st := stDefault
		if strings.HasPrefix(l, "error") || strings.Contains(l, "BAD") {
			st = stErr
		} else if strings.Contains(l, "differs") {
			st = stWarn
		}
		lines = append(lines, styledLine{[]styledSeg{{stDim, l[:7]}, {st, l[7:]}}})
	}
	lines = append(lines, styledLine{})

	if p.Valid() {
		switch u.view {
		case viewHex, viewDec:
			dec := u.view == viewDec
			cell := 3
			if dec {
				cell = 4
			}
			perRow := max(4, min(16, (w-8)/cell))
			for _, b := range fieldBlocks(p, dec, perRow) {
				lines = append(lines, line(stBold, b[0]))
				for _, l := range b[1:] {
					lines = append(lines, line(stDefault, l))
				}
			}
		case viewDump:
			perRow := 16
			if w < 4+16*3 {
				perRow = 8
			}
			lines = append(lines, dumpLines(p, perRow)...)
			lines = append(lines, styledLine{},
				styledLine{[]styledSeg{{stMagic, "magic "}, {stFieldA, "fields "}, {stFieldB, "fields "}, {stName, "name "}, {stDim, "unmapped"}}})
		}
	}

	u.scroll = max(0, min(u.scroll, len(lines)-h))
	for i := 0; i < h && u.scroll+i < len(lines); i++ {
		x := x0
		for _, seg := range lines[u.scroll+i].segs {
			x = u.str(x, y0+i, x0+w-x, seg.st, seg.s)
		}
	}
}

func dumpLines(p *Preset, perRow int) []styledLine {
	var out []styledLine
	for o := 0; o < presetSize; o += perRow {
		l := styledLine{[]styledSeg{{stDim, fmt.Sprintf("%02x  ", o)}}}
		for i := o; i < o+perRow; i++ {
			st := stDim
			switch fi := fieldAt[i]; {
			case fi < 0:
			case layout[fi].Label == "magic":
				st = stMagic
			case layout[fi].Label == "name":
				st = stName
			case fi%2 == 0:
				st = stFieldA
			default:
				st = stFieldB
			}
			l.segs = append(l.segs, styledSeg{st, fmt.Sprintf("%02x ", p.Data[i])})
		}
		out = append(out, l)
	}
	return out
}
