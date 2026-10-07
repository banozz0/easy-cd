#!/bin/sh
# record.sh re-makes demo/demo.mp4 and demo/demo.gif: it builds the fake
# home, builds ecd from source, records demo.tape with VHS, then adds the
# captions, key badges, zooms and arrows the tape's markers ask for.
# Needs go, vhs, ffmpeg and a Chrome or Chromium to draw the overlays
# (CHROME picks one). SKIP_RECORD=1 redoes the overlays on the last take.
set -eu
cd "$(dirname "$0")/.."
world=/tmp/ecd-demo
if [ -z "${SKIP_RECORD:-}" ]; then
  sh demo/fixture.sh "$world"
  go build -o "$world/bin/ecd" ./cmd/ecd
  vhs demo/demo.tape
fi

chrome=${CHROME:-}
for c in chrome-headless-shell chromium google-chrome \
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"; do
  [ -n "$chrome" ] && break
  command -v "$c" >/dev/null 2>&1 && chrome=$c
done
[ -n "$chrome" ] || { echo "record.sh: no Chrome found; set CHROME" >&2; exit 1; }
shot() { # shot HTML PNG WIDTH HEIGHT: a transparent screenshot of a page
  "$chrome" --headless --hide-scrollbars --default-background-color=00000000 \
    --window-size="$3,$4" --screenshot="$2" "file://$1" </dev/null >/dev/null 2>&1
}

out=$world/overlays
rm -rf "$out"; mkdir -p "$out"
# The tape's clock runs a little ahead of the take: each hidden clear and
# the capture itself lose a few frames. So each cut and each key snaps to
# the first screen change in its reach (a change, and the burst that follows
# a named key within 0.15 s, serves one event, in order); a key's badge shows one
# frame before its change, as the key is sent. Events between keys move with
# the next key. The reach (0.6 s back for a cut, 0.25 s back and 0.3 s on for
# a key) fits this tape's typing speed and pauses.
eval "$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate -of default=nw=1 "$world/raw.mp4")"
fps=$(echo "$r_frame_rate" | awk -F/ '{ print $1 / $2 }')
ffmpeg -v error -i "$world/raw.mp4" -vf "signalstats,metadata=print:key=lavfi.signalstats.YDIF:file=-" -f null - |
  paste - - | sed 's/.*pts_time:\([0-9.]*\).*YDIF=\(.*\)/\1 \2/' |
  awk '{ moved = $2 > 0.01 } moved && !was { print $1 } { was = moved }' > "$out/changes"
awk -f demo/timeline.awk demo/demo.tape | awk -F'\t' -v OFS='\t' -v frame="$(echo "1 / $fps" | bc -l)" '
  NR == FNR { change[++m] = $1; next }
  { n++; line[n] = $0; kind[n] = $1; at[n] = $2; typed[n] = $4 == "typed" }
  END {
    shift = 0
    for (i = 1; i <= n; i++) {
      if (kind[i] != "key" && kind[i] != "cut") continue
      lo = kind[i] == "cut" ? -0.6 : -0.25
      best = 0
      for (j = used + 1; j <= m && !best; j++) {
        d = change[j] - (at[i] + shift)
        if (d > 0.3) break
        if (d >= lo) best = j
      }
      if (best) {
        shift = change[best] - at[i]
        used = best
        if (!typed[i]) while (used < m && change[used + 1] - change[best] < 0.15) used++
      }
      off[i] = shift - (best && kind[i] == "key" ? frame : 0)
    }
    for (i = n; i >= 1; i--) if (kind[i] == "key" || kind[i] == "cut") next_ = off[i]; else off[i] = (next_ == "" ? shift : next_)
    for (i = 1; i <= n; i++) { $0 = line[i]; $2 = sprintf("%.3f", at[i] + off[i] < 0 ? 0 : at[i] + off[i]); print }
  }' "$out/changes" - > "$out/timeline"
strip=140
y=$((height - strip - 24))

# Every caption and badge is one row of a single tall page, so Chrome draws
# them all in one screenshot and ffmpeg crops each row back out. A caption
# lasts until the next one and fades; a badge, centred just above the
# caption, lasts until the next key, fading out only at a cut or the end.
# Each row: kind, start, stop, fade-out start (- for none), text.
awk -F'\t' '
  $1 == "end" { endT = $2 }
  $1 == "caption" || $1 == "key" || $1 == "cut" { n++; kind[n] = $1; at[n] = $2; text[n] = $3 }
  END {
    for (i = 1; i <= n; i++) {
      if (kind[i] == "cut") continue
      stop = endT; fade = 1
      for (j = i + 1; j <= n; j++) {
        if (kind[i] == "caption" && kind[j] == "caption") { stop = at[j]; break }
        if (kind[i] == "key" && kind[j] == "key") { stop = at[j]; fade = 0; break }
        if (kind[i] == "key" && kind[j] == "cut") { stop = at[j]; break }
      }
      printf "%s\t%.3f\t%.3f\t%s\t%s\n", kind[i], at[i], stop, fade ? sprintf("%.3f", stop - 0.2) : "-", text[i]
    }
  }' "$out/timeline" > "$out/rows"
rows=$(wc -l < "$out/rows" | tr -d ' ')
{
  cat <<EOF
<!doctype html><meta charset="utf-8">
<style>
  html, body { margin: 0; background: transparent; }
  div { height: ${strip}px; display: flex; align-items: center; justify-content: center; padding: 0 40px;
    box-sizing: border-box; font: 500 40px -apple-system, "Segoe UI", system-ui, sans-serif; }
  span { background: rgba(49, 50, 68, .95); color: #cdd6f4; padding: 16px 34px;
    border-radius: 18px; border: 2px solid #45475a; }
  .key span { font: 600 44px ui-monospace, Menlo, monospace; color: #f9e2af; border-color: #f9e2af; }
  b { color: #cba6f7; font-weight: 650; }
  code { font: 500 38px ui-monospace, Menlo, monospace; color: #a6e3a1; }
</style>
EOF
  while IFS='	' read -r kind start stop fade text; do
    [ "$kind" = key ] && text=$(printf '%s' "$text" | sed 's/&/\&amp;/g; s/</\&lt;/g')
    printf '<div class="%s"><span>%s</span></div>\n' "$kind" "$text"
  done < "$out/rows"
} > "$out/rows.html"
shot "$out/rows.html" "$out/rows.png" "$width" $((rows * strip))

# Each zoom eases from the factor before it over half a second, toward the
# top left where the picker draws.
z=$(awk -F'\t' '$1 == "zoom" {
    s = sprintf("clip((T-%s+0.25)/0.5,0,1)", $2)
    printf "+(%s-%s)*%s*%s*(3-2*%s)", $3, (prev == "" ? 1 : prev), s, s, s
    prev = $3
  }' "$out/timeline" | sed "s/T/(in\/$fps)/g")

graph="[0:v]scale=$((width * 2)):$((height * 2)):flags=lanczos,zoompan=z='1$z':x=0:y=0:d=1:s=${width}x${height}:fps=$fps[v0];"
graph="$graph[1:v]split=$rows$(i=1; while [ $i -le "$rows" ]; do printf '[r%d]' $i; i=$((i + 1)); done);"
i=0 last=v0
while IFS='	' read -r kind start stop fade text; do
  i=$((i + 1))
  f=""
  [ "$kind" = caption ] && f=",fade=in:st=$start:d=0.2:alpha=1"
  [ "$fade" != - ] && f="$f,fade=out:st=$fade:d=0.2:alpha=1"
  graph="$graph[r$i]crop=$width:$strip:0:$(((i - 1) * strip))$f[o$i];"
  at=$y; [ "$kind" = key ] && at=$((y - 100))
  graph="$graph[$last][o$i]overlay=0:$at:enable='between(t,$start,$stop)'[v$i];"
  last=v$i
done < "$out/rows"
inputs="-loop 1 -framerate $fps -i $out/rows.png"

# Arrows: each rings a box, or two boxes with a curve from the first to the
# second, then an optional number of seconds to show. The curve leaves
# sideways when the second box sits to the right, from the top when it sits
# above, and from the bottom otherwise; "via x,y ..." routes it through
# those points instead, round empty space.
awk -F'\t' '$1 == "arrow" { print $2 "\t" $3 }' "$out/timeline" > "$out/arrows"
i=0
while IFS='	' read -r at spec; do
  i=$((i + 1))
  stop=$(echo "$spec" | awk -v at="$at" -v w="$width" -v h="$height" -v html="$out/arrow$i.html" '{
    for (i = 1; i <= NF && $i != "via"; i++) num[++k] = $i
    for (i++; i <= NF; i++) { split($i, p, ","); vx[++v] = p[1]; vy[v] = p[2] }
    print at + (k % 4 ? num[k] : 1.9)
    x1 = num[1]; y1 = num[2]; w1 = num[3]; h1 = num[4]; x2 = num[5]; y2 = num[6]; w2 = num[7]; h2 = num[8]
    if (k >= 8) {
      if (v) {
        # Leave and arrive on the side facing the nearest waypoint, then round
        # each waypoint off with a curve through the midpoints.
        if (vx[1] > x1 + w1) { sx = x1 + w1 + 10; sy = clamp(vy[1], y1, y1 + h1) }
        else if (vy[1] < y1) { sx = clamp(vx[1], x1, x1 + w1); sy = y1 - 6 }
        else if (vy[1] > y1 + h1) { sx = clamp(vx[1], x1, x1 + w1); sy = y1 + h1 + 6 }
        else { sx = x1 - 10; sy = clamp(vy[1], y1, y1 + h1) }
        if (vy[v] > y2 + h2) { ex = clamp(vx[v], x2, x2 + w2); ey = y2 + h2 + 10 }
        else if (vy[v] < y2) { ex = clamp(vx[v], x2, x2 + w2); ey = y2 - 10 }
        else if (vx[v] > x2 + w2) { ex = x2 + w2 + 12; ey = clamp(vy[v], y2, y2 + h2) }
        else { ex = x2 - 12; ey = clamp(vy[v], y2, y2 + h2) }
        px[0] = sx; py[0] = sy; for (i = 1; i <= v; i++) { px[i] = vx[i]; py[i] = vy[i] }; px[v + 1] = ex; py[v + 1] = ey
        path = sprintf("M %d %d L %d %d", px[0], py[0], (px[0] + px[1]) / 2, (py[0] + py[1]) / 2)
        for (i = 1; i <= v; i++) path = path sprintf(" Q %d %d %d %d", px[i], py[i], (px[i] + px[i + 1]) / 2, (py[i] + py[i + 1]) / 2)
        path = path sprintf(" L %d %d", ex, ey)
      } else if (x2 > x1 + w1 + 40 && y2 < y1 + h1 && y2 + h2 > y1) {
        sx = x1 + w1 + 10; sy = y1 + h1 / 2; ex = x2 - 12; ey = sy < y2 + 30 ? y2 + 30 : sy
        mx = (sx + ex) / 2; path = sprintf("M %d %d C %d %d, %d %d, %d %d", sx, sy, mx, sy, mx, ey, ex, ey)
      } else if (y2 + h2 < y1) {
        sx = x1 + w1 / 2; sy = y1 - 6; ex = x2 + w2 / 2; ey = y2 + h2 + 10
        path = sprintf("M %d %d C %d %d, %d %d, %d %d", sx, sy, sx + (ex - sx) * 0.3, sy, ex, sy, ex, ey)
      } else {
        sx = x1 + w1 / 2; sy = y1 + h1 + 6; ex = x2 + w2 / 2; ey = y2 - 10; my = (sy + ey) / 2
        path = sprintf("M %d %d C %d %d, %d %d, %d %d", sx, sy, sx, my, ex, my, ex, ey)
      }
    }
    print "<!doctype html><style>html, body { margin: 0; background: transparent; }</style>" > html
    printf "<svg width=\"%d\" height=\"%d\" xmlns=\"http://www.w3.org/2000/svg\">\n", w, h > html
    print "<defs><marker id=\"head\" viewBox=\"0 0 10 10\" refX=\"8\" refY=\"5\" markerWidth=\"6\" markerHeight=\"6\" orient=\"auto\"><path d=\"M0 0 L10 5 L0 10 z\" fill=\"#fab387\"/></marker></defs>" > html
    print "<g fill=\"none\" stroke=\"#fab387\" stroke-width=\"5\" stroke-linecap=\"round\" stroke-linejoin=\"round\">" > html
    printf "<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"12\"/>\n", x1, y1, w1, h1 > html
    if (k >= 8) {
      printf "<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"12\"/>\n", x2, y2, w2, h2 > html
      printf "<path d=\"%s\" marker-end=\"url(#head)\"/>\n", path > html
    }
    print "</g></svg>" > html
  }
  function clamp(x, lo, hi) { return x < lo ? lo : x > hi ? hi : x }')
  shot "$out/arrow$i.html" "$out/arrow$i.png" "$width" "$height"
  inputs="$inputs -loop 1 -framerate $fps -i $out/arrow$i.png"
  graph="$graph[$((i + 1)):v]fade=in:st=$at:d=0.25:alpha=1,fade=out:st=$(echo "$stop - 0.3" | bc):d=0.3:alpha=1[a$i];"
  graph="$graph[$last][a$i]overlay=0:0:enable='between(t,$at,$stop)'[va$i];"
  last=va$i
done < "$out/arrows"

duration=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$world/raw.mp4")
ffmpeg -y -v error -i "$world/raw.mp4" $inputs -filter_complex "$graph[$last]format=yuv420p[out]" \
  -map "[out]" -t "$duration" -c:v libx264 -crf 20 -preset slow -movflags +faststart demo/demo.mp4

ffmpeg -y -v error -i demo/demo.mp4 -filter_complex "
  fps=10,scale=1200:-1:flags=lanczos,split[a][b];
  [a]palettegen=max_colors=128:stats_mode=diff[p];
  [b][p]paletteuse=dither=none:diff_mode=rectangle" demo/demo.gif
ls -l demo/demo.mp4 demo/demo.gif
