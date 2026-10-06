package picker

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// column is one column of the columns view: its visible rows, already
// scrolled and styled, and its share of the width.
type column struct {
	cells  []string
	weight int // share of the width; 0 is one cell wide
}

// maxTrail is how many levels the trail shows before the middle ones fold
// into an ellipsis column.
const maxTrail = 4

// ellipsis stands in for the folded middle of a deep trail.
var ellipsis = column{cells: []string{"…"}}

// path is dir and the folders above it, from home (or / outside home)
// down to dir.
func path(dir string) []string {
	root := "/"
	if home, _ := os.UserHomeDir(); home != "" && within(dir, home) {
		root = home
	}
	up := []string{dir}
	for d := dir; d != root && filepath.Dir(d) != d; d = filepath.Dir(d) {
		up = append(up, filepath.Dir(d))
	}
	slices.Reverse(up)
	return up
}

// trail is the columns left of dir, rows high: one per folder above it,
// the folder on the way down marked and kept in view, older ones narrower.
// Past maxTrail levels it keeps the first and the two nearest, with an
// ellipsis column between.
func trail(dir string, rows int) []column {
	p := path(dir)
	level := func(i, weight int) column {
		names, _ := entries(p[i])
		mark := slices.Index(names, filepath.Base(p[i+1]))
		off := inView(0, mark, rows, len(names))
		var cells []string
		for j := off; j < min(off+rows, len(names)); j++ {
			if j == mark {
				cells = append(cells, onPath.Render(names[j]))
			} else {
				cells = append(cells, names[j])
			}
		}
		return column{cells, weight}
	}
	n := len(p) - 1
	if n > maxTrail {
		return []column{level(0, 3), ellipsis, level(n-2, 5), level(n-1, 6)}
	}
	var cols []column
	for i := range n {
		cols = append(cols, level(i, i+3))
	}
	return cols
}

const colSep = " │ "

// columnsView renders up to rows rows of the trail, the current folder and
// the preview column side by side. The current folder and the preview weigh
// the most.
func (m model) columnsView(rows int) string {
	cols := trail(m.dir, rows)
	weight := len(cols) + 4
	return sideBySide(append(cols, column{m.window(rows, 0), weight}, column{m.preview.cells(rows), weight}), m.width)
}

// sideBySide lays cols out across width terminal columns (80 while it is
// unknown), each taking its weight's share.
func sideBySide(cols []column, width int) string {
	free, sum := cmp.Or(width, 80)-(len(cols)-1)*ansi.StringWidth(colSep), 0
	shown := 0
	for _, c := range cols {
		sum += c.weight
		shown = max(shown, len(c.cells))
		if c.weight == 0 {
			free--
		}
	}
	var b strings.Builder
	for r := range shown {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cell := ""
			if r < len(c.cells) {
				cell = c.cells[r]
			}
			cells[i] = fit(cell, max(free*c.weight/sum, 1))
		}
		b.WriteString(strings.TrimRight(strings.Join(cells, colSep), " ") + "\n")
	}
	return b.String()
}

// fit cuts s to w cells with an ellipsis, or pads it to w.
func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}
