// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

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
	u.popup(append(warning, "", "Press any key to continue."))
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
		in, ok := u.prompt(fmt.Sprintf("move to slot (%d-%d): ", minSlot, maxSlot), "")
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

// popup shows lines in a centered box until a key is pressed.
func (u *ui) popup(lines []string) {
	for {
		u.draw()
		w, h := u.s.Size()
		bw := 0
		for _, l := range lines {
			bw = max(bw, len([]rune(l)))
		}
		bw, bh := min(bw+4, w), min(len(lines)+2, h)
		x0, y0 := (w-bw)/2, (h-bh)/2
		st := stWarn.Bold(true)
		for y := y0; y < y0+bh; y++ {
			for x := x0; x < x0+bw; x++ {
				top, bottom, left, right := y == y0, y == y0+bh-1, x == x0, x == x0+bw-1
				r := ' '
				switch {
				case top && left:
					r = '┌'
				case top && right:
					r = '┐'
				case bottom && left:
					r = '└'
				case bottom && right:
					r = '┘'
				case top || bottom:
					r = '─'
				case left || right:
					r = '│'
				}
				u.s.SetContent(x, y, r, nil, st)
			}
		}
		for i, l := range lines {
			if 1+i < bh-1 {
				u.str(x0+2, y0+1+i, bw-4, st, l)
			}
		}
		u.s.Show()
		if _, ok := u.s.PollEvent().(*tcell.EventKey); ok {
			return
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
	u.str(1, 0, w-2, stHeader, fmt.Sprintf("kpm %s · %s · %d presets · %s · view: %s", version, u.dir, len(u.ps), marker, viewNames[u.view]))

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
		case p.SuffixState() == "!" || !p.MagicOK() || !p.SlotOK():
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
		} else if strings.Contains(l, "differs") || strings.Contains(l, "not loadable") {
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
