#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Convert an asciicast v2 recording of a full-screen terminal app into an
animated SVG that keeps the colours. Standard library only.

Supports the escape sequences a tcell app emits: cursor positioning, erase,
SGR (16/256/true colour, bold, dim, italic, underline, reverse); everything
else (modes, OSC, charset selection) is ignored.

usage: cast2svg.py IN.cast OUT.svg [--idle 2] [--font-size 14]
"""
import argparse
import html
import json
import re

# xterm default palette for colours 0-15, tuned for a dark background
BASE16 = [
    "#1d1f21", "#cc6666", "#b5bd68", "#f0c674", "#5f87d7", "#b294bb", "#8abeb7", "#c5c8c6",
    "#666666", "#d54e53", "#b9ca4a", "#e7c547", "#7aa6da", "#c397d8", "#70c0b1", "#eaeaea",
]
DEFAULT_FG, DEFAULT_BG = "#c5c8c6", "#1d1f21"


def color256(n):
    if n < 16:
        return BASE16[n]
    if n < 232:
        n -= 16
        steps = [0, 95, 135, 175, 215, 255]
        return "#%02x%02x%02x" % (steps[n // 36], steps[n // 6 % 6], steps[n % 6])
    v = 8 + (n - 232) * 10
    return "#%02x%02x%02x" % (v, v, v)


class Screen:
    def __init__(self, w, h):
        self.w, self.h = w, h
        self.x = self.y = 0
        self.reset_attr()
        self.cells = [[self.blank() for _ in range(w)] for _ in range(h)]
        self.pending = ""

    def reset_attr(self):
        self.fg = self.bg = None
        self.bold = self.dim = self.italic = self.underline = self.reverse = False

    def style(self):
        return (self.fg, self.bg, self.bold, self.dim, self.italic, self.underline, self.reverse)

    def blank(self):
        return (" ", (None, self.bg if hasattr(self, "bg") else None, False, False, False, False, False))

    def erase(self, y, x0, x1):
        for x in range(max(0, x0), min(self.w, x1)):
            self.cells[y][x] = (" ", (None, self.bg, False, False, False, False, False))

    def sgr(self, params):
        ps = [int(p) if p else 0 for p in params.split(";")] if params else [0]
        i = 0
        while i < len(ps):
            p = ps[i]
            if p == 0:
                self.reset_attr()
            elif p == 1:
                self.bold = True
            elif p == 2:
                self.dim = True
            elif p == 3:
                self.italic = True
            elif p == 4:
                self.underline = True
            elif p == 7:
                self.reverse = True
            elif p == 22:
                self.bold = self.dim = False
            elif p == 23:
                self.italic = False
            elif p == 24:
                self.underline = False
            elif p == 27:
                self.reverse = False
            elif 30 <= p <= 37:
                self.fg = BASE16[p - 30]
            elif 90 <= p <= 97:
                self.fg = BASE16[p - 90 + 8]
            elif 40 <= p <= 47:
                self.bg = BASE16[p - 40]
            elif 100 <= p <= 107:
                self.bg = BASE16[p - 100 + 8]
            elif p == 39:
                self.fg = None
            elif p == 49:
                self.bg = None
            elif p in (38, 48) and i + 1 < len(ps):
                if ps[i + 1] == 5 and i + 2 < len(ps):
                    c = color256(ps[i + 2])
                    i += 2
                elif ps[i + 1] == 2 and i + 4 < len(ps):
                    c = "#%02x%02x%02x" % tuple(ps[i + 2:i + 5])
                    i += 4
                else:
                    c = None
                if p == 38:
                    self.fg = c
                else:
                    self.bg = c
            i += 1

    def csi(self, priv, params, final):
        if priv:  # private modes (?1049h etc.)
            return
        nums = [int(p) if p else 0 for p in params.split(";")] if params else []
        if final == "H" or final == "f":
            row = (nums[0] if len(nums) > 0 and nums[0] else 1) - 1
            col = (nums[1] if len(nums) > 1 and nums[1] else 1) - 1
            self.y, self.x = min(max(row, 0), self.h - 1), min(max(col, 0), self.w - 1)
        elif final == "J":
            n = nums[0] if nums else 0
            if n == 2 or n == 3:
                for y in range(self.h):
                    self.erase(y, 0, self.w)
            elif n == 0:
                self.erase(self.y, self.x, self.w)
                for y in range(self.y + 1, self.h):
                    self.erase(y, 0, self.w)
        elif final == "K":
            n = nums[0] if nums else 0
            if n == 0:
                self.erase(self.y, self.x, self.w)
            elif n == 1:
                self.erase(self.y, 0, self.x + 1)
            else:
                self.erase(self.y, 0, self.w)
        elif final == "m":
            self.sgr(params)
        elif final == "A":
            self.y = max(0, self.y - (nums[0] if nums and nums[0] else 1))
        elif final == "B":
            self.y = min(self.h - 1, self.y + (nums[0] if nums and nums[0] else 1))
        elif final == "C":
            self.x = min(self.w - 1, self.x + (nums[0] if nums and nums[0] else 1))
        elif final == "D":
            self.x = max(0, self.x - (nums[0] if nums and nums[0] else 1))
        # everything else (t, l, h, r, ...) is ignored

    SEQ = re.compile(
        r"\x1b\[(?P<priv>[?>=!]?)(?P<params>[0-9;:]*)[ -/]*(?P<final>[@-~])"
        r"|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)"
        r"|\x1b[()*+][0-~]"
        r"|\x1b[^\[\]()*+]"
    )

    def feed(self, data):
        data = self.pending + data
        self.pending = ""
        i = 0
        n = len(data)
        while i < n:
            ch = data[i]
            if ch == "\x1b":
                m = self.SEQ.match(data, i)
                if not m:
                    if n - i < 64:  # incomplete, wait for the next chunk
                        self.pending = data[i:]
                        return
                    i += 1
                    continue
                if m.group("final"):
                    self.csi(m.group("priv"), m.group("params"), m.group("final"))
                i = m.end()
                continue
            if ch == "\r":
                self.x = 0
            elif ch == "\n":
                self.y = min(self.h - 1, self.y + 1)
            elif ch == "\b":
                self.x = max(0, self.x - 1)
            elif ch >= " ":
                if self.x < self.w:
                    self.cells[self.y][self.x] = (ch, self.style())
                self.x = min(self.w, self.x + 1)
            i += 1

    def snapshot(self):
        return tuple(tuple(row) for row in self.cells)


def frames_from_cast(path, idle, group=0.03):
    with open(path) as f:
        header = json.loads(f.readline())
        events = [json.loads(l) for l in f if l.strip()]
    scr = Screen(header["width"], header["height"])
    frames = []  # (time, snapshot)
    t_prev, t_out = 0.0, 0.0
    outs = [e for e in events if e[1] == "o"]
    for k, (t, _, data) in enumerate(outs):
        gap = t - t_prev
        t_prev = t
        t_out += min(gap, idle)
        scr.feed(data)
        nxt = outs[k + 1][0] if k + 1 < len(outs) else None
        if nxt is not None and nxt - t < group:
            continue  # the app is still drawing this frame
        snap = scr.snapshot()
        if not frames or frames[-1][1] != snap:
            frames.append((t_out, snap))
    # the app clears the screen when it quits: end on its last real frame
    while len(frames) > 1 and not any(c != " " for row in frames[-1][1] for c, _ in row):
        t_out = frames.pop()[0]
    return header, frames, t_out


def text_el(text, col, y, st, cw, lh, fs):
    """One <text> element for text starting at column col of row y."""
    fg, bg, bold, dim, italic, underline, rev = st
    color = (bg or DEFAULT_BG) if rev else (fg or DEFAULT_FG)
    attrs = ['x="%g"' % (col * cw), 'y="%g"' % (y * lh + fs), 'fill="%s"' % color]
    if bold:
        attrs.append('font-weight="bold"')
    if italic:
        attrs.append('font-style="italic"')
    if underline:
        attrs.append('text-decoration="underline"')
    if dim:
        attrs.append('opacity="0.55"')
    # fix the length, so fallback fonts (box drawing) cannot shift columns
    attrs.append('textLength="%g" lengthAdjust="spacingAndGlyphs"' % (len(text) * cw))
    return "<text %s>%s</text>" % (" ".join(attrs), html.escape(text, quote=False))


def render_frame(snap, cw, lh, fs):
    """SVG elements for one frame, positioned at y=0."""
    def eff_bg(st):
        fg, bg, rev = st[0], st[1], st[6]
        return (fg or DEFAULT_FG) if rev else bg

    out = []
    for y, row in enumerate(snap):
        # backgrounds: runs of equal non-default background
        x = 0
        while x < len(row):
            bg = eff_bg(row[x][1])
            x0 = x
            while x < len(row) and eff_bg(row[x][1]) == bg:
                x += 1
            if bg:
                out.append('<rect x="%g" y="%g" width="%g" height="%g" fill="%s"/>' % (x0 * cw, y * lh, (x - x0) * cw, lh, bg))
        # text: runs of equal style, one element per stretch of words, so
        # renderers that collapse spaces cannot shift the columns
        x = 0
        while x < len(row):
            st = row[x][1]
            x0 = x
            while x < len(row) and row[x][1] == st:
                x += 1
            text = "".join(c for c, _ in row[x0:x])
            for m in re.finditer(r"\S+(?: \S+)*", text):
                out.append(text_el(m.group(0), x0 + m.start(), y, st, cw, lh, fs))
    return "".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("cast")
    ap.add_argument("svg")
    ap.add_argument("--idle", type=float, default=2.0, help="cap pauses at this many seconds")
    ap.add_argument("--font-size", type=float, default=14)
    ap.add_argument("--hold", type=float, default=2.0, help="seconds to keep the last frame")
    a = ap.parse_args()

    header, frames, t_end = frames_from_cast(a.cast, a.idle)
    w, h = header["width"], header["height"]
    fs = a.font_size
    cw, lh = fs * 0.6, round(fs * 1.25, 2)
    pad, bar = 14, 30
    sw, sh = w * cw, h * lh
    total = t_end + a.hold

    # unique frames as defs, the film strip as <use> references
    uniq, ids = {}, []
    for _, snap in frames:
        if snap not in uniq:
            uniq[snap] = "f%d" % len(uniq)
        ids.append(uniq[snap])
    keyframes = []
    for i, (t, _) in enumerate(frames):
        keyframes.append("%.3f%%{transform:translateY(%gpx)}" % (100 * t / total, -i * sh))
    keyframes.append("100%%{transform:translateY(%gpx)}" % (-(len(frames) - 1) * sh))

    parts = []
    parts.append('<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" '
                 'width="%g" height="%g" viewBox="0 0 %g %g">' % (sw + 2 * pad, sh + 2 * pad + bar, sw + 2 * pad, sh + 2 * pad + bar))
    parts.append("<style>"
                 "text{font-family:'DejaVu Sans Mono','Cascadia Mono',Menlo,Consolas,'Liberation Mono',monospace;"
                 "font-size:%gpx;white-space:pre}"
                 ".strip{animation:play %.3fs steps(1,end) infinite}"
                 "@keyframes play{%s}"
                 "</style>" % (fs, total, "".join(keyframes)))
    parts.append('<rect width="100%%" height="100%%" rx="8" fill="%s"/>' % DEFAULT_BG)
    for i, c in enumerate(["#ff5f58", "#ffbd2e", "#18c132"]):
        parts.append('<circle cx="%g" cy="%g" r="6" fill="%s"/>' % (pad + 6 + i * 20, bar / 2 + 2, c))
    parts.append("<defs>")
    for snap, fid in uniq.items():
        parts.append('<g id="%s">%s</g>' % (fid, render_frame(snap, cw, lh, fs)))
    parts.append("</defs>")
    parts.append('<svg x="%g" y="%g" width="%g" height="%g" viewBox="0 0 %g %g">' % (pad, pad + bar, sw, sh, sw, sh))
    parts.append('<g class="strip">')
    for i, fid in enumerate(ids):
        parts.append('<use xlink:href="#%s" y="%g"/>' % (fid, i * sh))
    parts.append("</g></svg></svg>")
    with open(a.svg, "w") as f:
        f.write("".join(parts))
    print("%d frames (%d unique), %.1f s, %dx%d" % (len(frames), len(uniq), total, w, h))


if __name__ == "__main__":
    main()
