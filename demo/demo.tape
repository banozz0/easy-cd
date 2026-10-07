# The README demo. Run demo/record.sh, not this tape alone: it builds the
# fake home in /tmp/ecd-demo and ecd from source, records this tape, then
# adds what the marker comments ask for (demo/timeline.awk reads them):
#   # caption: TEXT                 a caption from here to the next one
#   # zoom FACTOR                   zoom toward the top left; 1 zooms out
#   # arrow X Y W H [X Y W H] [S] [via X,Y ...]
#                                   ring a box, or two boxes with an arrow from
#                                   the first to the second (through the via
#                                   points), in pixels of the final frame, for
#                                   S seconds (default 1.9)
# Every key press also gets a badge above the caption, up until the next key.

Output "/tmp/ecd-demo/raw.mp4"

Set Shell "bash"
Set FontSize 28
Set Width 1920
Set Height 1080
Set Padding 40
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set TypingSpeed 80ms
Set CursorBlink false
Set Framerate 30

# A clean shell in the fake home: no rc files from the recording machine.
Hide
Type "exec env -i HOME=/tmp/ecd-demo/home XDG_STATE_HOME=/tmp/ecd-demo/state XDG_CACHE_HOME=/tmp/ecd-demo/cache ZDOTDIR=/tmp/ecd-demo/zsh PATH=/tmp/ecd-demo/bin:/usr/bin:/bin TERM=xterm-256color LANG=en_US.UTF-8 zsh -i"
Enter
Sleep 1s
Type "clear"
Enter
Sleep 500ms
Show

# caption: Type <b>ecd</b>: your pinned and recent folders sit on top
Sleep 600ms
Type "ecd"
Sleep 250ms
Enter
Sleep 2.4s

# caption: <b>Arrows</b> walk the tree, <b>Enter</b> takes your shell there
Sleep 600ms
Down@330ms 5
Sleep 500ms
Right
Sleep 1.1s
Enter
Sleep 700ms
Type "pwd"
Sleep 200ms
Enter
Sleep 1.6s

Hide
Type "clear"
Enter
Sleep 300ms
Show

# caption: Press <b>4</b> → jump to website
Sleep 400ms
Type "ecd"
Sleep 200ms
Enter
Sleep 1s
# arrow 240 232 172 42 1.3
Sleep 1.3s
Type "4"
# arrow 240 232 172 42 32 534 330 44 2.2 via 326,292 590,300 590,556
Sleep 2.3s

Hide
Type "clear"
Enter
Sleep 300ms
Show

# caption: <b>^P</b> pins the highlighted folder
Sleep 400ms
Type "ecd"
Sleep 200ms
Enter
Sleep 1.2s
Left
Sleep 1.2s
Ctrl+P
# arrow 116 384 384 42 604 194 172 42 via 560,292 752,292
Sleep 2.3s

# caption: Just type to search every folder under home
# zoom 1.6
Type@200ms "web"
# arrow 360 318 100 52 498 436 212 58
Sleep 2s

# caption: <b>Tab</b> opens Finder-style columns along the path
# zoom 1
Sleep 1.5s
Tab
# arrow 32 498 154 42 492 386 660 38 via 430,519 430,405
Sleep 2.2s

# caption: <b>→</b> walks in: two levels down to the code
Sleep 300ms
Right
Sleep 900ms
Down
Sleep 700ms
Right
Sleep 1.4s

# caption: Highlight a file to preview it beside the list
Sleep 300ms
Down
# arrow 1004 348 206 38 1400 308 460 236
Sleep 1.7s
Down
# arrow 1004 386 260 38 1400 308 460 236
Sleep 1.9s

# caption: <b>→</b> reads the whole file, <b>↓</b> scrolls, <b>←</b> comes back
# zoom 1.25
Sleep 500ms
Right
Sleep 1.2s
Down@250ms 6
Sleep 1.2s
# zoom 1
Left
Sleep 900ms

# caption: <b>⇧←</b> jumps back home: the path reads <b>~</b>
Sleep 800ms
Shift+"←"
# arrow 70 272 290 228 64 84 40 36 2.6 via 28,386 28,102
Sleep 2.8s

# caption: <b>Esc</b> quits; your shell stays where it was
Sleep 800ms
Escape
Sleep 1.8s

# caption: <code>brew install banozz0/tap/easy-cd</code>
Sleep 2.4s
