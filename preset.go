// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Preset files

// fileRe matches impNNN.bin and impNNN_<name>.bin (also '-', ' ' or '.' as
// separator), and the same with exp: presets exported by the synth.
var fileRe = regexp.MustCompile(`(?i)^(imp|exp)(\d{3})(?:[_ .-](.*))?\.bin$`)

// errRequest marks an empty expNNN.bin: an export request the synth has not
// answered yet.
var errRequest = errors.New("export request, not exported yet")

type Preset struct {
	Path string
	Data []byte
	Err  error // set if the file is unreadable or not a preset
}

func loadPreset(path string) *Preset {
	p := &Preset{Path: path}
	p.Data, p.Err = os.ReadFile(path)
	if p.Err == nil && len(p.Data) == 0 && p.Kind() == "exp" {
		p.Err = errRequest
	} else if p.Err == nil && len(p.Data) != presetSize {
		p.Err = fmt.Errorf("size %d, want %d", len(p.Data), presetSize)
	}
	return p
}

func (p *Preset) File() string { return filepath.Base(p.Path) }
func (p *Preset) Valid() bool  { return p.Err == nil }

// Request reports whether the file is an unanswered export request.
func (p *Preset) Request() bool { return p.Err == errRequest }

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
	slot, _ = strconv.Atoi(m[2])
	return slot, m[3], true
}

// Kind is the filename prefix: "imp" (preset for the synth), "exp" (exported
// by the synth) or "" (not a preset filename).
func (p *Preset) Kind() string {
	m := fileRe.FindStringSubmatch(p.File())
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

// fileName builds impNNN.bin / impNNN_<suffix>.bin with the preset's prefix.
func (p *Preset) fileName(slot int, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%s%03d.bin", p.Kind(), slot)
	}
	return fmt.Sprintf("%s%03d_%s.bin", p.Kind(), slot, suffix)
}

const errNotPresetName = "%s: filename is not impNNN[_name].bin or expNNN[_name].bin"

func slotOK(s int) bool { return s >= minSlot && s <= maxSlot }

// SlotOK reports whether the filename's slot is one the synth can load
// (the init preset slot loads, but always as an empty preset).
func (p *Preset) SlotOK() bool {
	s, _, ok := p.Slot()
	return ok && slotOK(s) && s != initSlot
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
// in its folder. The synth deletes it after the import. expMarkerFile does
// the same for the export requests (empty expNNN.bin).
const (
	markerFile    = "imp.bin"
	expMarkerFile = "exp.bin"
)

func isMarker(path string) bool {
	b := filepath.Base(path)
	return strings.EqualFold(b, markerFile) || strings.EqualFold(b, expMarkerFile)
}

func hasFile(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func hasMarker(dir string) bool { return hasFile(dir, markerFile) }

// markers describes the markers present in dir.
func markers(dir string) []string {
	var l []string
	if hasMarker(dir) {
		l = append(l, "import marker "+markerFile)
	}
	if hasFile(dir, expMarkerFile) {
		l = append(l, "export marker "+expMarkerFile)
	}
	return l
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

// firmwareFile is a firmware waiting to be installed by the synth, not a
// preset.
const firmwareFile = "blink.bin"

func scanDir(dir string) ([]*Preset, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.bin"))
	if err != nil {
		return nil, err
	}
	var ps []*Preset
	for _, path := range paths {
		if !isMarker(path) && !strings.EqualFold(filepath.Base(path), firmwareFile) {
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
		return "", fmt.Errorf(errNotPresetName, p.File())
	}
	if p.Name() == "" {
		return "", fmt.Errorf("%s: preset has no name", p.File())
	}
	return p.fileName(slot, nameToFile(p.Name())), nil
}

func targetStrip(p *Preset) (string, error) {
	slot, _, ok := p.Slot()
	if !ok {
		return "", fmt.Errorf(errNotPresetName, p.File())
	}
	return p.fileName(slot, ""), nil
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
	if slot < minSlot || slot > maxSlot {
		return fmt.Errorf("slot %d out of range %d-%d", slot, minSlot, maxSlot)
	}
	cur, suf, ok := p.Slot()
	if !ok {
		return fmt.Errorf(errNotPresetName, p.File())
	}
	if cur == slot {
		return nil
	}
	for _, o := range all {
		if s, _, ok := o.Slot(); ok && s == slot && o.Kind() == p.Kind() && o != p {
			return fmt.Errorf("slot %03d is used by %s", slot, o.File())
		}
	}
	return p.renameFile(p.fileName(slot, suf))
}

// ---------------------------------------------------------------------------
// Transfers with the synth: export requests.

// parseSlots reads a slot list like "5", "1-10" or "3,7,9-12".
func parseSlots(s string) ([]int, error) {
	seen := map[int]bool{}
	var slots []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			return nil, fmt.Errorf("not a slot: %q", part)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil {
				return nil, fmt.Errorf("not a slot: %q", part)
			}
		}
		if a > b {
			return nil, fmt.Errorf("slots %q: start after end", part)
		}
		if !slotOK(a) || !slotOK(b) {
			return nil, fmt.Errorf("slots %q out of range %d-%d", part, minSlot, maxSlot)
		}
		for n := a; n <= b; n++ {
			if !seen[n] {
				seen[n] = true
				slots = append(slots, n)
			}
		}
	}
	sort.Ints(slots)
	return slots, nil
}

// usedSlots returns the slots that already have a file of the given kind in
// dir (with or without a name in the filename).
func usedSlots(dir, kind string) (map[int]string, error) {
	ps, err := scanDir(dir)
	if err != nil {
		return nil, err
	}
	used := map[int]string{}
	for _, p := range ps {
		if s, _, ok := p.Slot(); ok && p.Kind() == kind {
			used[s] = p.File()
		}
	}
	return used, nil
}

// targetRequests returns the export request files to create in dir and the
// slots skipped because a file for them exists already.
func targetRequests(dir string, slots []int) (create []string, skipped []string, err error) {
	used, err := usedSlots(dir, "exp")
	if err != nil {
		return nil, nil, err
	}
	for _, s := range slots {
		if f, ok := used[s]; ok {
			skipped = append(skipped, f)
		} else {
			create = append(create, fmt.Sprintf("exp%03d.bin", s))
		}
	}
	return create, skipped, nil
}

// opRequests creates the empty export requests and the export marker.
func opRequests(dir string, create []string) error {
	for _, f := range create {
		fh, err := os.OpenFile(filepath.Join(dir, f), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		fh.Close()
	}
	return createMarker(dir, expMarkerFile)
}

func createMarker(dir, name string) error {
	if hasFile(dir, name) {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, name), nil, 0o644)
}
