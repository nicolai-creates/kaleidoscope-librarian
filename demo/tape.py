#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Play a vhs tape (https://github.com/charmbracelet/vhs) into a tmux session.

Supports the commands demo/kalib.tape uses: Type, Type@<delay>, Sleep, Hide,
Show, and the keys Enter, Tab, Escape, Space, Backspace, Up, Down, Left,
Right (with an optional @<delay> and count, as in vhs). Output, Set and
Require are vhs settings and ignored; Hide ... Show is skipped, because
record.sh starts the program itself.

usage: tape.py TAPE TMUX_SESSION
"""
import re
import shlex
import subprocess
import sys
import time

KEYS = {"Enter": "Enter", "Tab": "Tab", "Escape": "Escape", "Space": "Space", "Backspace": "BSpace",
        "Up": "Up", "Down": "Down", "Left": "Left", "Right": "Right"}


def duration(s):
    m = re.fullmatch(r"([0-9.]+)(ms|s)?", s)
    if not m:
        raise ValueError("bad duration %r" % s)
    return float(m[1]) / 1000 if m[2] == "ms" else float(m[1])


def main():
    tape, session = sys.argv[1], sys.argv[2]

    def tmux(*args):
        subprocess.run(["tmux", "send-keys", "-t", session, *args], check=True)

    hidden = False
    for n, line in enumerate(open(tape), 1):
        words = shlex.split(line, comments=True)
        if not words:
            continue
        cmd, _, delay = words[0].partition("@")
        delay = duration(delay) if delay else 0
        args = words[1:]
        if cmd in ("Hide", "Show"):
            hidden = cmd == "Hide"
        elif cmd in ("Output", "Set", "Require") or hidden:
            continue
        elif cmd == "Sleep":
            time.sleep(duration(args[0]))
        elif cmd == "Type":
            for ch in args[0] if delay else [args[0]]:
                tmux("-l", ch)
                time.sleep(delay)
        elif cmd in KEYS:
            for _ in range(int(args[0]) if args else 1):
                tmux(KEYS[cmd])
                time.sleep(delay)
        else:
            sys.exit("%s:%d: unsupported command %s" % (tape, n, cmd))


if __name__ == "__main__":
    main()
