// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ---------------------------------------------------------------------------
// TUI

const (
	viewDevice = iota
	viewParams
	viewDump
	numViews
)

var viewNames = [numViews]string{"device", "parameters", "hex dump"}

type ui struct {
	s         tcell.Screen
	dir       string
	ps        []*Preset
	cur, top  int
	view      int
	dec       bool // parameters view: decimal after hex
	jump      int  // section to scroll to on the next draw: 0 = File, 1-7 = group, -1 = none
	scroll    int
	lw        int // width of the preset list, set by draw
	status    string
	statusErr bool
	copyDir   string // last target folder of 'c'
	fetchDir  string // last target folder of 'b'
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, rest)
		}
	}
	return p
}

func sameDir(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && x == y
}

// Bar colors from nicolai-creates.art: white on dark blue for the header,
// dark blue on orange for the footer.
var (
	colBlue   = tcell.NewHexColor(0x0232a0)
	colOrange = tcell.NewHexColor(0xef9f00)
	colErr    = tcell.NewHexColor(0x723400)
)

var (
	stDefault = tcell.StyleDefault
	stHeader  = stDefault.Background(colBlue).Foreground(tcell.ColorWhite)
	stFooter  = stDefault.Background(colOrange).Foreground(colBlue)
	stDim     = stDefault.Foreground(tcell.ColorGray)
	stBold    = stDefault.Bold(true)
	stErr     = stDefault.Foreground(tcell.ColorRed)
	stWarn    = stDefault.Foreground(tcell.ColorYellow)
	stName    = stDefault.Foreground(tcell.ColorGreen).Bold(true)
	stMagic   = stDefault.Foreground(tcell.ColorBlue)
	stFieldA  = stDefault.Foreground(tcell.ColorAqua)
	stFieldB  = stDefault.Foreground(tcell.ColorFuchsia)
	stSteps   = stDefault.Foreground(tcell.ColorOrange)
)

// shortHelp is the status line; viewHelp adds the keys of the current view.
const shortHelp = "↑↓ move  Tab/v view  J/K scroll  r rename  ? help  q quit"

var viewHelp = [numViews]string{"  1-7 group", "  d dec  0-7 section", ""}

var longHelp = []string{
	"kalib keys",
	"",
	"↑ ↓  j k  PgUp PgDn  Home End   move in the preset list",
	"Tab  v                          next view: device, parameters, hex dump",
	"J  K                            scroll the info pane",
	"mouse wheel                     scrolls the pane under the pointer",
	"d                               parameters view: decimal values on/off",
	"1-7                             jump to a page group (parameters, device)",
	"0                               jump to bytes outside any page (parameters)",
	"",
	"r                               rename the preset",
	"n / N                           add the preset name to the filename (selected / all)",
	"f / F                           set the preset name from the filename (selected / all)",
	"s / S                           strip the name from the filename (selected / all)",
	"m                               move the preset to another slot",
	"i                               create or remove the imp.bin import marker",
	"e                               ask the synth to export slots (empty expNNN.bin + exp.bin)",
	"b                               fetch exported presets (expNNN.bin) to a folder, e.g. a backup",
	"X                               delete export files (expNNN.bin, exp.bin, expc.bin)",
	"c                               copy the preset to a folder for import into a slot",
	"D                               open the synth's drive (KLRU)",
	"E                               eject the synth's drive",
	"g                               reload the folder",
	"? h                             this help",
	"q  Esc                          quit",
}

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
	s.EnableMouse(tcell.MouseButtonEvents)
	u := &ui{s: s, dir: dir, ps: ps, dec: true, jump: -1}
	if len(ps) == 0 {
		u.setErr("no .bin files in " + dir)
	}
	u.popup(append(warning, "", "Press any key to continue."))
	for {
		u.draw()
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			if !u.handleKey(ev) {
				return nil
			}
		case *tcell.EventMouse:
			u.handleMouse(ev)
		}
	}
}

// handleMouse scrolls the pane under the pointer with the wheel: the preset
// list moves the selection, the info pane scrolls.
func (u *ui) handleMouse(ev *tcell.EventMouse) {
	d := 0
	switch {
	case ev.Buttons()&tcell.WheelUp != 0:
		d = -1
	case ev.Buttons()&tcell.WheelDown != 0:
		d = 1
	default:
		return
	}
	if x, _ := ev.Position(); x < u.lw {
		u.move(d)
	} else {
		u.scroll = max(0, u.scroll+3*d)
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
	case 'd':
		u.dec = !u.dec
	case '0', '1', '2', '3', '4', '5', '6', '7':
		u.jump = int(r - '0')
	case '?', 'h':
		u.popup(longHelp)
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
		name, ok := u.ask([]string{"Rename " + p.File() + ":", "a-z, 0-9 and space, up to 11 characters"}, p.Name())
		if !ok || name == p.Name() {
			return true
		}
		old := p.Name()
		u.result(opRename(p, name), fmt.Sprintf("renamed %q -> %q", old, p.Name()))
	case 'n':
		u.single(p, opNameToFile, "name added to filename")
	case 'N':
		u.all("Add preset name to all filenames", opNameToFile)
	case 'f':
		if p != nil && p.Valid() {
			if t, err := targetFileToName(p); err == nil && t != p.Name() &&
				!u.confirm(fmt.Sprintf("Set name %q -> %q?", p.Name(), t)) {
				return true
			}
		}
		u.single(p, opFileToName, "name taken from filename")
	case 'F':
		u.all("Set stored name from filename for all files", opFileToName)
	case 's':
		u.single(p, opStrip, "filename reset to impNNN.bin")
	case 'S':
		u.all("Strip names from all filenames", opStrip)
	case 'e':
		in, ok := u.ask([]string{"Request export of slots:", fmt.Sprintf("e.g. 5, 1-10, 3,7 or all (%d-%d)", minSlot, maxSlot)}, "")
		if !ok || strings.TrimSpace(in) == "" {
			return true
		}
		slots, err := parseSlots(in)
		if err != nil {
			u.setErr(err.Error())
			return true
		}
		if old, err := targetClean(u.dir); err == nil && len(old) > 0 {
			switch u.askList([]string{"Delete old export files first?", "Slots with an export file are not requested again."},
				old, "y  delete first   n  keep   Esc  cancel", "yn") {
			case 'y':
				if err := opClean(u.dir, old); err != nil {
					u.reload()
					u.setErr(err.Error())
					return true
				}
			case 0:
				return true
			}
		}
		create, skipped, err := targetRequests(u.dir, slots)
		if err == nil {
			err = opRequests(u.dir, create)
		}
		u.reload()
		u.result(err, fmt.Sprintf("%d export requests created, %d skipped (exist); marker %s set", len(create), len(skipped), expMarkerFile))
	case 'X':
		files, err := targetClean(u.dir)
		if err != nil || len(files) == 0 {
			u.result(err, "no export files in "+u.dir)
			return true
		}
		if u.askList([]string{"Delete these export files from " + u.dir + "?"}, files, "y  delete   n  keep", "yn") != 'y' {
			return true
		}
		err = opClean(u.dir, files)
		u.reload()
		u.result(err, fmt.Sprintf("%d export files deleted", len(files)))
	case 'b':
		if u.fetchDir == "" {
			u.fetchDir = os.Getenv(fetchEnv)
		}
		to, ok := u.askFolder("Fetch exported presets to folder:", u.fetchDir)
		if !ok {
			return true
		}
		u.fetchDir = to
		fetch, skipped, err := targetFetch(u.dir, to)
		if err == nil && len(fetch) == 0 {
			err = fmt.Errorf("nothing to fetch from %s", u.dir)
		}
		if err == nil {
			err = opFetch(to, fetch)
		}
		if err == nil && len(skipped) > 0 {
			u.askList([]string{fmt.Sprintf("%d presets copied to %s, %d skipped:", len(fetch), to, len(skipped))},
				skipped, "any key  continue", "")
		}
		u.result(err, fmt.Sprintf("%d presets copied to %s, %d skipped", len(fetch), to, len(skipped)))
	case 'c':
		if p == nil || !p.Valid() {
			return true
		}
		if u.copyDir == "" {
			u.copyDir, _ = findDevice()
		}
		dir, ok := u.askFolder("Copy "+p.File()+" to folder:", u.copyDir)
		if !ok {
			return true
		}
		u.copyDir = dir
		def := ""
		if s, _, ok := p.Slot(); ok {
			def = strconv.Itoa(s)
		}
		in, ok := u.ask([]string{"Import " + p.File() + " into slot:", fmt.Sprintf("%d-%d", minSlot, maxSlot)}, def)
		if !ok {
			return true
		}
		n, err := strconv.Atoi(strings.TrimSpace(in))
		if err != nil {
			u.setErr("not a number: " + in)
			return true
		}
		path, err := opImport(p, dir, n)
		if err == nil && sameDir(dir, u.dir) {
			u.reload()
		}
		u.result(err, fmt.Sprintf("copied to %s, marker %s set", path, markerFile))
	case 'D':
		dir, ok := u.device()
		if !ok {
			return true
		}
		u.dir = dir
		u.reload()
		u.setMsg("opened the synth's drive " + dir)
	case 'E':
		dir, ok := u.device()
		if !ok {
			return true
		}
		if !u.confirm("Eject the synth's drive " + dir + "?") {
			return true
		}
		note, err := ejectDevice(dir)
		if note != "" {
			note = " (" + note + ")"
		}
		u.result(err, "ejected "+dir+": safe to unplug"+note)
	case 'm':
		if p == nil {
			return true
		}
		in, ok := u.ask([]string{"Move " + p.File() + " to slot:", fmt.Sprintf("%d-%d", minSlot, maxSlot)}, "")
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
	if !u.confirm(what + "?") {
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

// editLine applies a key to a line being typed: done is set on Enter
// (ok true) and Esc (ok false).
func editLine(buf []rune, ev *tcell.EventKey) (out []rune, done, ok bool) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return nil, true, false
	case tcell.KeyEnter:
		return buf, true, true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(buf) > 0 {
			buf = buf[:len(buf)-1]
		}
	case tcell.KeyCtrlU:
		buf = buf[:0]
	case tcell.KeyRune:
		buf = append(buf, ev.Rune())
	}
	return buf, false, false
}

// ask asks for a line of text in a popup: head, then an input line starting
// with initial. Returns false on Esc.
func (u *ui) ask(head []string, initial string) (string, bool) {
	buf := []rune(initial)
	for {
		w, _ := u.s.Size()
		fw := max(1, min(max(50, len(buf)+1), w-8)) // input width
		shown := buf[max(0, len(buf)-(fw-1)):]
		lines := append(append([]string{}, head...), "", string(shown)+strings.Repeat(" ", fw-len(shown)), "", "Enter  ok   Esc  cancel   Ctrl-U  clear")
		x0, y0 := u.drawBox(lines)
		u.s.ShowCursor(x0+2+len(shown), y0+2+len(head))
		u.s.Show()
		ev, ok := u.s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue
		}
		var done bool
		if buf, done, ok = editLine(buf, ev); done {
			u.s.HideCursor()
			return string(buf), ok
		}
	}
}

// askFolder asks for a folder in a popup, starting with initial. Returns
// false on Esc or an empty answer; ~/ is expanded.
func (u *ui) askFolder(q, initial string) (string, bool) {
	in, ok := u.ask([]string{q}, initial)
	dir := strings.TrimSpace(in)
	return expandHome(dir), ok && dir != ""
}

// popup shows lines in a centered box until a key is pressed.
// device finds the synth's drive, or explains in a popup why it can't.
func (u *ui) device() (string, bool) {
	dir, err := findDevice()
	switch {
	case err == errNoDevice:
		u.popup([]string{
			"Synth not found",
			"",
			"No drive named " + deviceLabel + " is mounted.",
			"Connect the synth via USB, or set " + deviceEnv + " to its folder.",
			"",
			"Press any key to continue.",
		})
	case err != nil:
		u.popup([]string{"Synth not found", "", err.Error(), "", "Press any key to continue."})
	}
	return dir, err == nil
}

// popup shows lines in a box until a key is pressed.
func (u *ui) popup(lines []string) {
	for {
		u.drawBox(lines)
		if _, ok := u.s.PollEvent().(*tcell.EventKey); ok {
			return
		}
	}
}

// drawBox draws the screen with lines in a centred box on top and returns
// the box's top left corner.
func (u *ui) drawBox(lines []string) (x0, y0 int) {
	u.draw()
	w, h := u.s.Size()
	bw := 0
	for _, l := range lines {
		bw = max(bw, len([]rune(l)))
	}
	bw, bh := min(bw+4, w), min(len(lines)+2, h)
	x0, y0 = (w-bw)/2, (h-bh)/2
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
	return x0, y0
}

// listShort is how many items askList shows before "+ show all".
const listShort = 5

// askList shows head, a list of items and foot in a popup. It shows the first
// few items; + shows all of them, scrolled with ↑↓ PgUp PgDn. It returns the
// key pressed from keys, or 0 for Esc. With keys empty, any other key
// returns 0.
func (u *ui) askList(head, items []string, foot, keys string) rune {
	all, off := false, 0
	for {
		_, h := u.s.Size()
		rows := min(len(items), listShort)
		if all {
			rows = max(1, min(len(items), h-len(head)-6))
		}
		off = max(0, min(off, len(items)-rows))
		if !all {
			off = 0
		}
		lines := append(append([]string{}, head...), "")
		for _, it := range items[off : off+rows] {
			lines = append(lines, "  "+it)
		}
		switch {
		case all && rows < len(items):
			lines = append(lines, fmt.Sprintf("  %d-%d of %d   ↑↓ scroll   + fewer", off+1, off+rows, len(items)))
		case all:
			lines = append(lines, "  + fewer")
		case rows < len(items):
			lines = append(lines, fmt.Sprintf("  … %d more   + show all", len(items)-rows))
		}
		u.drawBox(append(lines, "", foot))
		ev, ok := u.s.PollEvent().(*tcell.EventKey)
		if !ok {
			continue
		}
		switch ev.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			return 0
		case tcell.KeyUp:
			off--
			continue
		case tcell.KeyDown:
			off++
			continue
		case tcell.KeyPgUp:
			off -= rows
			continue
		case tcell.KeyPgDn:
			off += rows
			continue
		case tcell.KeyRune:
			r := ev.Rune()
			switch {
			case r == '+' && len(items) > listShort:
				all, off = !all, 0
				continue
			case r == 'k':
				off--
				continue
			case r == 'j':
				off++
				continue
			case strings.ContainsRune(keys, r):
				return r
			}
		}
		if keys == "" {
			return 0
		}
	}
}

// confirm asks a yes/no question in a popup; Esc means no.
func (u *ui) confirm(q string) bool {
	for {
		u.drawBox([]string{q, "", "y  yes      n  no"})
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
	if m := markers(u.dir); len(m) > 0 {
		marker = strings.Join(m, ", ") + " present"
	}
	view := viewNames[u.view]
	if u.view == viewParams && u.dec {
		view += " +dec"
	}
	u.str(1, 0, w-2, stHeader, fmt.Sprintf("kalib %s · %s · %d presets · %s · view: %s", version, u.dir, len(u.ps), marker, view))

	lw := min(listWidth, w/2)
	u.lw = lw
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
		case p.Request():
			st = stDim
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

	u.clearRow(h-1, stFooter)
	switch {
	case u.status == "":
		u.str(0, h-1, w, stFooter, shortHelp+viewHelp[u.view])
	case u.statusErr:
		u.str(0, h-1, w, stFooter.Foreground(colErr).Bold(true), u.status)
	default:
		u.str(0, h-1, w, stFooter.Bold(true), u.status)
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
		} else if strings.Contains(l, "differs") || strings.Contains(l, "not loadable") || strings.Contains(l, "init preset") {
			st = stWarn
		}
		if len(lines) == 0 {
			st = stBold
		}
		lines = append(lines, line(st, l))
	}
	lines = append(lines, styledLine{})

	var groupStart []int
	if p.Valid() {
		switch u.view {
		case viewParams:
			groupStart = paramLines(p, u.dec, &lines)
		case viewDevice:
			groupStart = deviceLines(p, w, &lines)
		case viewDump:
			perRow := 16
			if w < 4+16*3 {
				perRow = 8
			}
			lines = append(lines, dumpLines(p, perRow)...)
			lines = append(lines, styledLine{},
				styledLine{[]styledSeg{{stMagic, "magic "}, {stFieldA, "fields "}, {stFieldB, "fields "}, {stName, "name "}, {stSteps, "steps "}, {stDim, "unmapped"}}})
		}
	}

	// groupStart holds the groups, then (parameters view) the File section.
	switch j := u.jump; {
	case j == 0 && len(groupStart) > len(synthGroups):
		u.scroll = groupStart[len(synthGroups)]
	case j > 0 && j <= len(synthGroups) && j <= len(groupStart):
		u.scroll = groupStart[j-1]
	}
	u.jump = -1
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
			case layout[fi].Label == "sSEQ-VALU":
				st = stSteps
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

// paramLines appends the parameters in display order (group, page, encoder),
// then the fields outside any page, and returns where each group starts.
func paramLines(p *Preset, dec bool, lines *[]styledLine) (groupStart []int) {
	val := func(f field) string {
		b := p.Data[f.Off : f.Off+f.Len]
		if f.Len > 1 {
			return hexBytes(b)
		}
		if dec {
			return fmt.Sprintf("%02x %-6s", b[0], fmt.Sprintf("(%d)", b[0]))
		}
		return fmt.Sprintf("%02x", b[0])
	}
	bytesW := 2 // hex byte width in multi-byte rows, 3 to line up with decimals
	if dec {
		bytesW = 3
	}
	add := func(page string, f field) {
		off := fmt.Sprintf("0x%02x", f.Off)
		if f.Len > 1 {
			off += fmt.Sprintf("-0x%02x", f.Off+f.Len-1)
		}
		width := 2
		if dec {
			width = 9
		}
		segs := []styledSeg{
			{stBold, fmt.Sprintf("  %-5s %-5s ", page, paramName(f.Label))},
			{stDim, fmt.Sprintf("%-5s ", off)},
		}
		if f.Len > 1 {
			*lines = append(*lines, styledLine{append(segs, styledSeg{stDim, f.Note})})
			b := p.Data[f.Off : f.Off+f.Len]
			steps := f.Label == "sSEQ-VALU"
			w := bytesW
			if steps {
				w = 4 // room for a sign and three digits, see the value row
			}
			l := "              " + padBytes(b, w)
			if f.Label == "name" {
				l += "   \"" + p.Name() + "\""
			}
			*lines = append(*lines, line(stDefault, l))
			if dec {
				ds := decBytes(b)
				if steps {
					ds = padInts(b, w, func(c byte) int { return int(c) })
				}
				*lines = append(*lines, line(stDefault, "              "+ds))
			}
			if steps {
				vals := padInts(b, w, func(c byte) int { return int(int8(c - 0x40)) })
				st := stDefault
				if !hasStepMod(p) {
					st = stDim // leftover data, saved before v76
				}
				*lines = append(*lines, line(st, "              "+vals))
			}
			return
		}
		segs = append(segs, styledSeg{stDefault, fmt.Sprintf("%-*s  ", width, val(f))}, styledSeg{stDim, f.Note})
		*lines = append(*lines, styledLine{segs})
	}
	for gi, g := range synthGroups {
		groupStart = append(groupStart, len(*lines))
		*lines = append(*lines, line(stBold, fmt.Sprintf("%d %s", gi+1, g.Name)))
		for _, pg := range g.Pages {
			name := pg.Name
			for _, l := range pg.Enc {
				if fi, ok := fieldByLabel[l]; ok {
					add(name, layout[fi])
					name = ""
				}
			}
		}
		*lines = append(*lines, styledLine{})
	}
	groupStart = append(groupStart, len(*lines))
	*lines = append(*lines, line(stBold, "0 File"))
	for _, f := range unpagedFields() {
		if f.Label != "magic" { // shown in the hex dump
			add("", f)
		}
	}
	return groupStart
}

// sparkLevels draws step values as bars, 0 in the middle.
var sparkLevels = []rune("▁▂▃▄▅▆▇")

// deviceLines appends the pages as on the synth display: one row per page,
// one column per encoder, values as shown on the synth where the encoding is
// known and dimmed raw values where it is not.
func deviceLines(p *Preset, w int, lines *[]styledLine) (groupStart []int) {
	cw := max(10, min(14, (w-6)/4))
	stepMod := hasStepMod(p)
	for gi, g := range synthGroups {
		groupStart = append(groupStart, len(*lines))
		*lines = append(*lines, line(stBold, fmt.Sprintf("%d %s", gi+1, g.Name)))
		for _, pg := range g.Pages {
			l := styledLine{[]styledSeg{{stBold, fmt.Sprintf("%-6s", pg.Name)}}}
			if len(pg.Enc) == 0 {
				l.segs = append(l.segs, styledSeg{stDim, "(display only)"})
			}
			for _, lab := range pg.Enc {
				name := fmt.Sprintf("%-4s", paramName(lab))
				val, st := "·", stDim
				if fi, ok := fieldByLabel[lab]; ok {
					f := layout[fi]
					switch {
					case !stepMod && (pg.Name == "sSEQ" || pg.Name == "sMX"):
						val = "-"
					case lab == "name":
						val, st = p.Name(), stDefault
					case f.Len > 1:
						val = "▾"
					default:
						var known bool
						val, known = displayValue(lab, p.Data[f.Off])
						if known {
							st = stDefault
						}
					}
				}
				vw := max(1, cw-len(name)-2)
				if lab == "name" {
					vw = nameLen
				}
				if r := []rune(val); len(r) > vw {
					val = string(r[:vw])
				}
				l.segs = append(l.segs, styledSeg{stDim, name + " "}, styledSeg{st, fmt.Sprintf("%-*s ", vw, val)})
			}
			*lines = append(*lines, l)
			switch {
			case pg.Name != "sSEQ":
			case stepMod:
				*lines = append(*lines, stepLines(p)...)
			default:
				*lines = append(*lines, line(stDim, "      not stored: preset saved before firmware v76"))
			}
		}
		*lines = append(*lines, styledLine{})
	}
	return groupStart
}

// stepLines shows the 16 step values as bars and numbers; steps beyond the
// sequence length are dimmed. Bars are scaled to the largest step.
func stepLines(p *Preset) []styledLine {
	f := layout[fieldByLabel["sSEQ-VALU"]]
	n := int(p.Data[layout[fieldByLabel["sSEQ-LEN"]].Off])
	if n < 1 || n > f.Len {
		n = f.Len
	}
	steps := p.Data[f.Off : f.Off+f.Len]
	top := 1
	for _, b := range steps {
		top = max(top, abs(int(int8(b-0x40))))
	}
	mid := len(sparkLevels) / 2
	bars := styledLine{[]styledSeg{{stDim, "      "}}}
	nums := styledLine{[]styledSeg{{stDim, "      "}}}
	for i, b := range steps {
		v := int(int8(b - 0x40))
		lv := mid + (v*mid*2+top)/(2*top)
		if v < 0 {
			lv = mid - (-v*mid*2+top)/(2*top)
		}
		st := stDefault
		if i >= n {
			st = stDim
		}
		bars.segs = append(bars.segs, styledSeg{st, string(sparkLevels[lv])})
		if i < n {
			nums.segs = append(nums.segs, styledSeg{stDefault, fmt.Sprintf("%d ", v)})
		}
	}
	bars.segs = append(bars.segs, styledSeg{stDim, fmt.Sprintf("  %d steps", n)})
	return []styledLine{bars, nums}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// padInts formats f(byte) as decimals, each right-aligned in w columns.
func padInts(b []byte, w int, f func(byte) int) string {
	s := make([]string, len(b))
	for i, c := range b {
		s[i] = fmt.Sprintf("%*d", w, f(c))
	}
	return strings.Join(s, " ")
}

// padBytes formats bytes as hex, each right-aligned in w columns.
func padBytes(b []byte, w int) string {
	s := make([]string, len(b))
	for i, c := range b {
		s[i] = fmt.Sprintf("%*s", w, fmt.Sprintf("%02x", c))
	}
	return strings.Join(s, " ")
}
