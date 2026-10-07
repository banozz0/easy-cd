# timeline.awk reads demo.tape and prints when each visible thing happens in
# the recorded video, one tab-separated event per line:
#   key      SECONDS  LABEL [typed] a key press, as its badge reads; typed
#                                   marks a character of typed text
#   caption  SECONDS  TEXT          from a "# caption: TEXT" line
#   zoom     SECONDS  FACTOR        from "# zoom FACTOR"; 1 ends a zoom
#   arrow    SECONDS  BOXES         from "# arrow ...": one box to ring, or a
#                                   box and the one it points at, then an
#                                   optional duration
#   cut      SECONDS                where a hidden stretch ends
#   end      SECONDS
# Time runs as VHS runs the tape: Sleep waits, each key or typed character
# waits the typing speed after it, and nothing hidden is recorded.

function secs(d) {
  if (d ~ /ms$/) return substr(d, 1, length(d) - 2) / 1000
  if (d ~ /s$/) return substr(d, 1, length(d) - 1) + 0
  return d + 0
}

# press records a key at the clock and moves the clock on by speed. Presses
# of the same key in a row count up on one badge: ↓, ↓ ×2, ↓ ×3.
function press(label, speed) {
  if (!hidden) {
    if (label == last && t - lastT < 1.2) count++
    else count = 1
    printf "key\t%.3f\t%s\n", t, label (count > 1 ? " ×" count : "")
    last = label; lastT = t
    t += speed
  }
}

BEGIN { speed = 0.05 }

$1 == "Set" && $2 == "TypingSpeed" { speed = secs($3) }
$1 == "Hide" { hidden = 1 }
$1 == "Show" { hidden = 0; printf "cut\t%.3f\n", t }
$1 == "Sleep" { if (!hidden) t += secs($2) }

/^# caption: / { sub(/^# caption: /, ""); printf "caption\t%.3f\t%s\n", t, $0; next }
$1 == "#" && $2 == "zoom" { printf "zoom\t%.3f\t%s\n", t, $3 }
$1 == "#" && $2 == "arrow" { sub(/^# arrow /, ""); printf "arrow\t%.3f\t%s\n", t, $0; next }

$1 ~ /^Type/ {
  s = speed
  if ($1 ~ /@/) s = secs(substr($1, index($1, "@") + 1))
  text = $0
  sub(/^[^"]*"/, "", text); sub(/"[^"]*$/, "", text)
  typed = ""
  for (i = 1; i <= length(text); i++) {
    typed = typed substr(text, i, 1)
    if (!hidden) { printf "key\t%.3f\t%s\ttyped\n", t, typed; t += s }
  }
  last = ""
  next
}

$1 ~ /^(Down|Up|Left|Right|Enter|Tab|Escape|Backspace|Ctrl\+|Shift\+)/ {
  name = $1; s = speed; n = ($2 ~ /^[0-9]+$/) ? $2 : 1
  if (name ~ /@/) { s = secs(substr(name, index(name, "@") + 1)); name = substr(name, 1, index(name, "@") - 1) }
  label = name
  if (name == "Down") label = "↓"
  else if (name == "Up") label = "↑"
  else if (name == "Left") label = "←"
  else if (name == "Right") label = "→"
  else if (name == "Escape") label = "Esc"
  else if (name ~ /^Ctrl\+/) label = "^" substr(name, 6)
  else if (name ~ /^Shift\+/) { label = substr(name, 7); gsub(/"/, "", label); label = "⇧" label }
  for (i = 0; i < n; i++) press(label, s)
}

END { printf "end\t%.3f\n", t }
