package picker_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/banozz0/easy-cd/internal/picker"
)

var (
	tab       = tea.KeyMsg{Type: tea.KeyTab}
	shiftLeft = tea.KeyMsg{Type: tea.KeyShiftLeft}
)

// inColumns starts the picker in dir, presses keys then Tab, waits for the
// columns, quits with Esc and returns the last screen.
func inColumns(t *testing.T, dir string, keys ...tea.KeyMsg) string {
	t.Helper()
	tm := newPicker(t, dir)
	for _, k := range append(keys, tab) {
		tm.Send(k)
	}
	waitColumns(t, tm)
	tm.Send(esc)
	return tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()
}

// waitColumns waits until the terminal shows the columns view.
func waitColumns(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	waitFor(t, tm, "│")
}

// firstRow is the first row of the columns, one cell per column, highlight
// marker and padding removed, empty cells dropped.
func firstRow(t *testing.T, screen string) []string {
	t.Helper()
	for _, line := range strings.Split(screen, "\n") {
		if !strings.Contains(line, "│") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(line, "│") {
			if c = strings.TrimLeft(strings.TrimSpace(c), "▸ "); c != "" {
				cells = append(cells, c)
			}
		}
		return cells
	}
	t.Fatalf("no columns on screen:\n%s", screen)
	return nil
}

func TestColumnsShowEveryLevelFromHome(t *testing.T) {
	home := newHome(t, "alpha/bravo/charlie/delta")

	screen := inColumns(t, home, right, right, right) // into charlie

	// ~ lists alpha, alpha lists bravo, bravo lists charlie, charlie lists delta
	if got, want := firstRow(t, screen), []string{"alpha", "bravo", "charlie", "delta"}; !slices.Equal(got, want) {
		t.Fatalf("columns %q, want %q, screen:\n%s", got, want, screen)
	}
}

func TestColumnsFoldTheMiddleAncestorsPastFour(t *testing.T) {
	for _, tc := range []struct {
		levels int
		want   []string
	}{
		{4, []string{"one", "two", "three", "four", "five"}},
		{5, []string{"one", "…", "four", "five", "six"}},
		{6, []string{"one", "…", "five", "six", "seven"}},
	} {
		t.Run(strings.Join(tc.want, " "), func(t *testing.T) {
			home := newHome(t, "one/two/three/four/five/six/seven")
			screen := inColumns(t, home, slices.Repeat([]tea.KeyMsg{right}, tc.levels)...)

			if got := firstRow(t, screen); !slices.Equal(got, tc.want) {
				t.Fatalf("columns %q, want %q, screen:\n%s", got, tc.want, screen)
			}
		})
	}
}

func TestTabTwiceBringsTheListBackWithTheSameHighlight(t *testing.T) {
	home := newHome(t, "a", "b", "c")
	tm := newPicker(t, home)

	tm.Send(down)
	tm.Send(tab)
	waitColumns(t, tm)
	tm.Send(tab)
	tm.Send(enter)

	final := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
	if screen := final.View(); strings.Contains(screen, "│") || !strings.Contains(screen, "▸ b\n") {
		t.Fatalf("want the list with b highlighted, screen:\n%s", screen)
	}
	var stdout bytes.Buffer
	picker.Finish(final, &stdout)
	if want := filepath.Join(home, "b") + "\n"; stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestArrowsWalkTheSameInColumns(t *testing.T) {
	home := newHome(t, "a/b/c", "m/n")

	out, code := run(t, home, tab, down, right, left, enter)

	if want := filepath.Join(home, "m") + "\n"; out != want || code != 0 {
		t.Fatalf("stdout %q exit %d, want %q exit 0", out, code, want)
	}
}

func TestShiftLeftJumpsBackToHome(t *testing.T) {
	for name, keys := range map[string][]tea.KeyMsg{
		"list":      {shiftLeft, enter},
		"columns":   {tab, shiftLeft, enter},
		"searching": append(typed("zz"), shiftLeft, enter),
	} {
		t.Run(name, func(t *testing.T) {
			home := newHome(t, "a", "m/n/o/p")

			out, _ := run(t, filepath.Join(home, "m", "n", "o"), keys...)

			// Back home the highlight starts at the top, on a.
			if want := filepath.Join(home, "a") + "\n"; out != want {
				t.Fatalf("stdout %q, want %q", out, want)
			}
		})
	}
}

func TestColumnsKeepTheWayDownInView(t *testing.T) {
	var dirs []string
	for i := range 40 {
		dirs = append(dirs, fmt.Sprintf("f%02d", i))
	}
	home := newHome(t, append(dirs, "f39/inner")...)

	screen := inColumns(t, filepath.Join(home, "f39", "inner"))

	// ~ lists 40 folders on a 24-row terminal; f39 is the one on the way down.
	for _, line := range strings.Split(screen, "\n") {
		if first, _, ok := strings.Cut(line, "│"); ok && strings.TrimSpace(first) == "f39" {
			return
		}
	}
	t.Fatalf("the ~ column lacks f39, screen:\n%s", screen)
}
