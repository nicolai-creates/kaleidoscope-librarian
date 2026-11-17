# The README demo, in vhs tape syntax (https://github.com/charmbracelet/vhs).
# demo/record.sh plays it into kalib with demo/tape.py and records an
# animated SVG; the Hide ... Show part and the settings are only for vhs,
# which can make a GIF from this tape too.

Output kalib.gif

Set Shell "bash"
Set FontSize 14
Set Width 1000
Set Height 476
Set Padding 14
Set WindowBar Colorful
Set BorderRadius 8
Set TypingSpeed 0
Set CursorBlink false

Hide
Type "./kalib 'Factory Presets'"
Enter
Sleep 1s
Show

Sleep 3s
Type "x"                          # close the start popup
Sleep 1.2s
Type "j"
Sleep 500ms
Type "j"                          # select spacehunt
Sleep 2.8s
Type "2"                          # jump to page groups
Sleep 1.5s
Type "7"
Sleep 2s
Type "1"
Sleep 1.2s
Type "v"                          # long form
Sleep 2.5s
Type@250ms "JJJJJJ"               # scroll
Sleep 1.5s
Type "v"                          # hex dump
Sleep 3s
Type "v"                          # back to "as on device"
Sleep 1s
Type@600ms "jjj"
Sleep 1.5s
Type "?"                          # help
Sleep 3.5s
Type "x"
Sleep 2s
