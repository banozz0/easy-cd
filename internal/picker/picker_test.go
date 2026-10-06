package picker_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"github.com/banozz0/easy-cd/internal/picker"
)

// newHome builds a temp home holding the given folders and points HOME and
// the XDG state and cache dirs into the test's own temp space.
func newHome(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	for _, d := range append([]string{""}, dirs...) {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	return home
}

// newPicker runs picker.New(dir) under teatest, on an 80x24 terminal unless
// opts say otherwise. Its cleanup waits for the index
// refresh, which outlives the program, before the temp dirs go.
func newPicker(t *testing.T, dir string, opts ...teatest.TestOption) *teatest.TestModel {
	t.Helper()
	m := picker.New(dir)
	t.Cleanup(func() { picker.WaitRefresh(m) })
	return teatest.NewTestModel(t, m, append([]teatest.TestOption{teatest.WithInitialTermSize(80, 24)}, opts...)...)
}

var (
	up    = tea.KeyMsg{Type: tea.KeyUp}
	down  = tea.KeyMsg{Type: tea.KeyDown}
	right = tea.KeyMsg{Type: tea.KeyRight}
	left  = tea.KeyMsg{Type: tea.KeyLeft}
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	esc   = tea.KeyMsg{Type: tea.KeyEsc}
	pin   = tea.KeyMsg{Type: tea.KeyCtrlP}

	backspace = tea.KeyMsg{Type: tea.KeyBackspace}
)

// waitFor waits until the terminal has shown every one of want.
func waitFor(t *testing.T, tm *teatest.TestModel, want ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		for _, w := range want {
			if !bytes.Contains(out, []byte(w)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(5*time.Second))
}

// typed is s typed on the keyboard, one key per character.
func typed(s string) []tea.KeyMsg {
	var keys []tea.KeyMsg
	for _, r := range s {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return keys
}

// stateFile is the path of a file in the ecd state dir newHome set up.
func stateFile(home, name string) string {
	return filepath.Join(filepath.Dir(home), "state", "ecd", name)
}

// run starts the picker in dir, sends keys, and returns what ecd would print
// on stdout and its exit code.
func run(t *testing.T, dir string, keys ...tea.KeyMsg) (string, int) {
	t.Helper()
	tm := newPicker(t, dir)
	for _, k := range keys {
		tm.Send(k)
	}
	var stdout bytes.Buffer
	code := picker.Finish(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)), &stdout)
	return stdout.String(), code
}

func TestLeftReturnsHighlightAndEnterPrintsIt(t *testing.T) {
	home := newHome(t, "a/b/c", "m/n")

	out, code := run(t, home, down, right, left, enter)

	if want := filepath.Join(home, "m") + "\n"; out != want || code != 0 {
		t.Fatalf("stdout %q exit %d, want %q exit 0", out, code, want)
	}
}

func TestEscPrintsNothingAndFails(t *testing.T) {
	home := newHome(t, "a/b/c")

	out, code := run(t, home, right, esc)

	if out != "" || code == 0 {
		t.Fatalf("stdout %q exit %d, want empty and non-zero", out, code)
	}
}

func TestBreadcrumbShowsWhereYouAre(t *testing.T) {
	home := newHome(t, "a/b/c")
	tm := newPicker(t, home)

	tm.Send(right)
	tm.Send(right)

	waitFor(t, tm, "~ › a › b")
	tm.Send(esc)
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestFourJumpsToTheMostVisitedFolder(t *testing.T) {
	home := newHome(t, "x", "y")
	x, y := filepath.Join(home, "x"), filepath.Join(home, "y")
	for range 3 {
		run(t, x, enter) // x has no subfolders, so Enter lands on x
	}
	run(t, y, enter)

	out, code := run(t, home, typed("4")...)

	if want := x + "\n"; out != want || code != 0 {
		t.Fatalf("stdout %q exit %d, want %q exit 0", out, code, want)
	}
}

func TestOnlyRealDestinationsAreLogged(t *testing.T) {
	home := newHome(t)
	tmp := filepath.Join(filepath.Dir(home), "tmp")
	t.Setenv("TMPDIR", tmp)

	run(t, home, enter) // an empty home: Enter lands on home itself
	for _, d := range []string{filepath.Join(tmp, "scratch"), filepath.Join(home, "a", "b")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	run(t, tmp, enter) // lands on tmp/scratch
	if _, err := os.Stat(stateFile(home, "visits")); !os.IsNotExist(err) {
		t.Fatalf("visit log exists after landing on home and a temp folder: %v", err)
	}

	run(t, home, right, enter) // walks through a, lands on a/b
	log, err := os.ReadFile(stateFile(home, "visits"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(string(log)), "\n"); len(lines) != 1 || !strings.HasSuffix(lines[0], "\t"+filepath.Join(home, "a", "b")) {
		t.Fatalf("visit log %q, want one line for a/b", log)
	}
}

func TestCtrlPPinsTheHighlightedFolderOnSlotOne(t *testing.T) {
	home := newHome(t, "a", "b")
	b := filepath.Join(home, "b")

	run(t, home, down, pin, esc)
	out, _ := run(t, home, typed("1")...)
	if out != b+"\n" {
		t.Fatalf("1 printed %q, want %q", out, b)
	}

	run(t, home, down, pin, esc)
	pins, _ := os.ReadFile(stateFile(home, "pins"))
	if strings.Contains(string(pins), b) {
		t.Fatalf("pins file still holds b after the second Ctrl+P: %q", pins)
	}
}

func TestDigitAfterALetterIsSearchText(t *testing.T) {
	home := newHome(t, "a", "b")
	run(t, home, down, pin, esc) // slot 1 holds b
	tm := newPicker(t, home)

	for _, k := range typed("a1") {
		tm.Send(k)
	}

	waitFor(t, tm, "a1")
	tm.Send(esc) // clears the search
	tm.Send(esc)
	var stdout bytes.Buffer
	picker.Finish(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)), &stdout)
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q, want nothing: the 1 jumped", stdout.String())
	}
}

func TestFourthPinIsRefusedWithANotice(t *testing.T) {
	home := newHome(t, "a", "b", "c", "d")
	tm := newPicker(t, home)

	for _, k := range []tea.KeyMsg{pin, down, pin, down, pin, down, pin} {
		tm.Send(k)
	}

	waitFor(t, tm, "★ pinned  1 a  2 b  3 c", "3 pins already")
	tm.Send(esc)
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	pins, _ := os.ReadFile(stateFile(home, "pins"))
	if want := strings.Join([]string{"a", "b", "c", ""}, "\n"); strings.ReplaceAll(string(pins), home+"/", "") != want {
		t.Fatalf("pins file %q, want a, b and c only", pins)
	}
}

func TestListScrollsToKeepTheHighlightOnScreen(t *testing.T) {
	var names []string
	for i := range 40 {
		names = append(names, fmt.Sprintf("list/f%02d", i))
	}
	home := newHome(t, names...)
	tm := newPicker(t, filepath.Join(home, "list"))

	for range 39 {
		tm.Send(down)
	}
	tm.Send(enter)

	final := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
	// Bubble Tea splits the view on newlines and drops lines off the top past
	// the terminal height, so the breadcrumb only reaches the terminal when
	// the view fits.
	screen, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(screen, []byte("~ › list")) || !bytes.Contains(screen, []byte("▸ 📁 f39")) {
		t.Fatalf("terminal output lacks the breadcrumb or the highlighted f39:\n%s", screen)
	}
	if rows := strings.Split(final.View(), "\n"); len(rows) > 24 {
		t.Fatalf("view is %d rows on a 24-row terminal", len(rows))
	}
	var stdout bytes.Buffer
	picker.Finish(final, &stdout)
	if want := filepath.Join(home, "list", "f39") + "\n"; stdout.String() != want {
		t.Fatalf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestDefaultLookHasIconsAndTheCalmTheme(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor) // what a modern terminal reports
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	home := newHome(t, "code")
	writeFile(t, home, "notes.txt", "x\n")
	tm := newPicker(t, home)

	waitFor(t, tm, "notes.txt")
	tm.Send(esc)
	screen := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()

	// A folder icon, a file icon, the breadcrumb and highlight marker in the
	// theme's blue (#89b4fa as Lip Gloss renders it), the highlighted row on
	// its #313244 bar (as Lip Gloss renders it) and the key help at the bottom, with no flags set.
	for _, want := range []string{"📁", "📄", "\x1b[1;38;2;137;179;250m~", "\x1b[38;2;137;179;250m▸", "\x1b[48;2;48;50;68m", "esc quit"} {
		if !strings.Contains(screen, want) && !strings.Contains(ansi.Strip(screen), want) {
			t.Fatalf("default screen lacks %q:\n%q", want, screen)
		}
	}
	// The bar spans the terminal's width.
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, "▸") && ansi.StringWidth(line) != 80 {
			t.Fatalf("highlighted row is %d wide, want 80: %q", ansi.StringWidth(line), line)
		}
	}
}

func TestVisitLogKeepsItsNewestLinesPastTheCap(t *testing.T) {
	home := newHome(t, "old", "new")
	var log strings.Builder
	for i := range 5000 {
		fmt.Fprintf(&log, "%d\t%s\n", 1_000_000+i, filepath.Join(home, "old"))
	}
	if err := os.MkdirAll(filepath.Dir(stateFile(home, "visits")), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Dir(stateFile(home, "visits")), "visits", log.String())

	run(t, filepath.Join(home, "new"), enter)

	data, err := os.ReadFile(stateFile(home, "visits"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 2500 || !strings.HasSuffix(lines[len(lines)-1], "\t"+filepath.Join(home, "new")) || strings.HasPrefix(lines[0], "1000000\t") {
		t.Fatalf("visit log has %d lines from %q to %q, want at most 2500 ending in the new visit, the oldest gone", len(lines), lines[0], lines[len(lines)-1])
	}
}
