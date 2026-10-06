package picker_test

import (
	"bytes"
	"os"
	"path/filepath"
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

// run starts the picker in dir, sends keys, and returns what ecd would print
// on stdout and its exit code.
func run(t *testing.T, dir string, keys ...tea.KeyType) (string, int) {
	t.Helper()
	tm := teatest.NewTestModel(t, picker.New(dir), teatest.WithInitialTermSize(80, 24))
	for _, k := range keys {
		tm.Send(tea.KeyMsg{Type: k})
	}
	var stdout bytes.Buffer
	code := picker.Finish(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)), &stdout)
	return stdout.String(), code
}

func TestLeftReturnsHighlightAndEnterPrintsIt(t *testing.T) {
	home := newHome(t, "a/b/c", "m/n")

	out, code := run(t, home, tea.KeyDown, tea.KeyRight, tea.KeyLeft, tea.KeyEnter)

	if want := filepath.Join(home, "m") + "\n"; out != want || code != 0 {
		t.Fatalf("stdout %q exit %d, want %q exit 0", out, code, want)
	}
}

func TestEscPrintsNothingAndFails(t *testing.T) {
	home := newHome(t, "a/b/c")

	out, code := run(t, home, tea.KeyRight, tea.KeyEsc)

	if out != "" || code == 0 {
		t.Fatalf("stdout %q exit %d, want empty and non-zero", out, code)
	}
}

func TestBreadcrumbShowsWhereYouAre(t *testing.T) {
	home := newHome(t, "a/b/c")
	tm := teatest.NewTestModel(t, picker.New(home), teatest.WithInitialTermSize(80, 24))

	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	teatest.WaitFor(t, tm.Output(), func(screen []byte) bool {
		return bytes.Contains(screen, []byte("~ › a › b"))
	}, teatest.WithDuration(5*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
