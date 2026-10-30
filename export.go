// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

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
		SavedSlot int      `json:"saved_slot"`
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
			Name: p.Name(), File: p.File(), Slot: slot, SavedSlot: int(p.Data[slotOff]),
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
	fmt.Println("# off  field     distinct  min  max  values (count), most common first")
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
		fmt.Printf("0x%02x  %-9s %8d  %3d  %3d  %s\n", o, label, len(counts), lo, hi, strings.Join(top, " "))
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
