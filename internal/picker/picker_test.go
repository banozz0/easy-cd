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
	"github.com/charmbracelet/x/exp/teatest"

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

// newPicker runs picker.New(dir) under teatest. Its cleanup waits for the
// index refresh, which outlives the program, before the temp dirs go.
func newPicker(t *testing.T, dir string) *teatest.TestModel {
	t.Helper()
	m := picker.New(dir)
	t.Cleanup(func() { picker.WaitRefresh(m) })
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
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

	teatest.WaitFor(t, tm.Output(), func(screen []byte) bool {
		return bytes.Contains(screen, []byte("~ › a › b"))
	}, teatest.WithDuration(5*time.Second))
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

	teatest.WaitFor(t, tm.Output(), func(screen []byte) bool {
		return bytes.Contains(screen, []byte("a1"))
	}, teatest.WithDuration(5*time.Second))
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

	teatest.WaitFor(t, tm.Output(), func(screen []byte) bool {
		return bytes.Contains(screen, []byte("★ pinned  1 a  2 b  3 c")) &&
			bytes.Contains(screen, []byte("3 pins already"))
	}, teatest.WithDuration(5*time.Second))
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
	if !bytes.Contains(screen, []byte("~ › list")) || !bytes.Contains(screen, []byte("▸ f39")) {
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
