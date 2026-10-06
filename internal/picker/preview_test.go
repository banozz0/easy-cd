package picker_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

// writeFile writes data to name under dir.
func writeFile(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// previewed starts the picker in dir, presses keys, waits until the terminal
// shows want, quits with Esc and returns the last screen.
func previewed(t *testing.T, dir, want string, keys ...tea.KeyMsg) string {
	t.Helper()
	tm := newPicker(t, dir)
	for _, k := range keys {
		tm.Send(k)
	}
	waitFor(t, tm, want)
	tm.Send(esc)
	return tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()
}

func TestHighlightedTextFileShowsItsFirstLinesBesideTheList(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "plan.txt", "first line\n\tsecond line\n")

	screen := previewed(t, filepath.Join(home, "docs"), "second line")

	if got, want := firstRow(t, screen), []string{"plan.txt", "first line"}; !slices.Equal(got, want) {
		t.Fatalf("row %q, want %q, screen:\n%s", got, want, screen)
	}
}

func TestHighlightedTextFileFillsThePreviewColumn(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "plan.txt", "first line\n")

	screen := previewed(t, filepath.Join(home, "docs"), "first line", tab)

	// ~ lists docs, docs lists plan.txt, the preview column shows its line.
	if got := firstRow(t, screen); len(got) != 3 || got[1] != "plan.txt" || got[2] != "first line" {
		t.Fatalf("columns %q, want docs, plan.txt, first line; screen:\n%s", got, screen)
	}
}

func TestBinaryFileShowsANoticeAndNoneOfItsBytes(t *testing.T) {
	home := newHome(t, "bin")
	writeFile(t, filepath.Join(home, "bin"), "blob", "MAGIC\x00\x01\x02PAYLOAD\n")

	screen := previewed(t, filepath.Join(home, "bin"), "binary")

	if strings.Contains(screen, "MAGIC") || strings.Contains(screen, "PAYLOAD") {
		t.Fatalf("binary bytes on screen:\n%s", screen)
	}
}

func TestOversizedFileShowsANoticeInsteadOfContent(t *testing.T) {
	home := newHome(t, "big")
	writeFile(t, filepath.Join(home, "big"), "huge.log", "HEAD\n")
	if err := os.Truncate(filepath.Join(home, "big", "huge.log"), 11<<20); err != nil {
		t.Fatal(err)
	}

	screen := previewed(t, filepath.Join(home, "big"), "too large")

	if strings.Contains(screen, "HEAD") {
		t.Fatalf("oversized file content on screen:\n%s", screen)
	}
}

func TestPreviewDropsTerminalControlCodes(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "evil.txt", "safe\x1b]2;pwned\x07 text\r\n")

	screen := previewed(t, filepath.Join(home, "docs"), "text")

	if strings.ContainsAny(screen, "\x1b\x07\r") {
		t.Fatalf("control codes reached the screen: %q", screen)
	}
}

func TestFilesListDimmedAfterFoldersAndStayUnreadUntilHighlighted(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	home := newHome(t, "yak", "zeta")
	writeFile(t, home, "alpha.txt", "ALPHA-CONTENT\n")
	writeFile(t, home, "beta.txt", "BETA-CONTENT\n")
	tm := newPicker(t, home)

	var first []byte
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		first = out
		return bytes.Contains(out, []byte("beta.txt"))
	}, teatest.WithDuration(5*time.Second))
	if bytes.Contains(first, []byte("CONTENT")) {
		t.Fatalf("file content on screen before a file was highlighted:\n%s", first)
	}
	if bytes.Index(first, []byte("zeta")) > bytes.Index(first, []byte("alpha.txt")) {
		t.Fatalf("want folders above files:\n%s", first)
	}
	if !bytes.Contains(first, []byte("\x1b[2malpha.txt\x1b[0m")) {
		t.Fatalf("want alpha.txt dimmed: %q", first)
	}
	tm.Send(down)
	tm.Send(down) // onto alpha.txt
	waitFor(t, tm, "ALPHA-CONTENT")
	tm.Send(esc)
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestEnterRightAndPinDoNothingOnAFile(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "plan.txt", "x\n")

	out, code := run(t, filepath.Join(home, "docs"), enter, right, pin, enter, esc)

	if out != "" || code == 0 {
		t.Fatalf("stdout %q exit %d, want empty and non-zero", out, code)
	}
	if _, err := os.Stat(stateFile(home, "pins")); !os.IsNotExist(err) {
		t.Fatalf("pins file written for a file: %v", err)
	}
}
