#!/bin/bash
# SPDX-License-Identifier: MIT
#
# Records the README demo (demo/kalib.svg): runs kalib on a temporary copy of
# "Factory Presets" in tmux, plays demo/kalib.tape into it while asciinema
# records, and converts the recording to an animated SVG.
# Needs: go, tmux, asciinema (3.x), python3. Run from the repository root.
set -euo pipefail

cols=104 rows=24
session=kalib-demo
root=$(pwd)
tmp=$(mktemp -d)
trap 'tmux kill-session -t $session 2>/dev/null || true; rm -rf "$tmp"' EXIT

go build -o "$tmp/kalib" .
cp -r "Factory Presets" "$tmp/"

tmux new-session -d -s $session -x $cols -y $rows -c "$tmp" \
	"env -u COLORTERM TERM=xterm-256color asciinema rec -q --overwrite -f asciicast-v2 -i 2 -c './kalib \"Factory Presets\"' demo.cast"
python3 -I "$root/demo/tape.py" "$root/demo/kalib.tape" $session
tmux send-keys -t $session -l q
for _ in $(seq 50); do  # wait until kalib has quit and asciinema has written the file
	tmux has-session -t $session 2>/dev/null || break
	timeout 0.1 tail -f /dev/null || true
done

python3 -I "$root/demo/cast2svg.py" "$tmp/demo.cast" "$root/demo/kalib.svg"
