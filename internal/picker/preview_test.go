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
	"github.com/charmbracelet/x/ansi"
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

// writeScript writes an executable shell script named name under dir and
// returns its path.
func writeScript(t *testing.T, dir, name, script string) string {
	t.Helper()
	writeFile(t, dir, name, "#!/bin/sh\n"+script)
	path := filepath.Join(dir, name)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
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

	if got, want := firstRow(t, screen), []string{"plan.txt", "preview"}; !slices.Equal(got, want) {
		t.Fatalf("row %q, want %q, screen:\n%s", got, want, screen)
	}
}

func TestHighlightedTextFileFillsThePreviewColumn(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "plan.txt", "first line\n")

	screen := previewed(t, filepath.Join(home, "docs"), "first line", tab)

	// ~ lists docs, docs lists plan.txt, the preview column is labelled.
	if got := firstRow(t, screen); len(got) != 3 || got[1] != "plan.txt" || got[2] != "preview" {
		t.Fatalf("columns %q, want docs, plan.txt, preview; screen:\n%s", got, screen)
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

func TestPinDoesNothingOnAFile(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "plan.txt", "x\n")

	run(t, filepath.Join(home, "docs"), pin, esc)

	if _, err := os.Stat(stateFile(home, "pins")); !os.IsNotExist(err) {
		t.Fatalf("pins file written for a file: %v", err)
	}
}

func TestPreviewLabelSitsAboveAFilePreviewAndNeverOverAFolder(t *testing.T) {
	for _, view := range []struct {
		name string
		keys []tea.KeyMsg
	}{{"list", nil}, {"columns", []tea.KeyMsg{tab}}} {
		t.Run(view.name, func(t *testing.T) {
			lipgloss.SetColorProfile(termenv.ANSI)
			t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
			home := newHome(t, "docs/sub")
			writeFile(t, filepath.Join(home, "docs"), "plan.txt", "first line\n")

			onFolder := previewed(t, filepath.Join(home, "docs"), "sub", view.keys...)
			onFile := previewed(t, filepath.Join(home, "docs"), "first line", append(view.keys, down)...)

			if strings.Contains(onFolder, "preview") {
				t.Fatalf("label over a folder:\n%s", onFolder)
			}
			if !strings.Contains(onFile, "\x1b[2mpreview\x1b[0m") {
				t.Fatalf("want a dim preview label: %q", onFile)
			}
			lines := strings.Split(ansi.Strip(onFile), "\n")
			at := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "│ preview") })
			if at < 0 || at+1 == len(lines) || !strings.HasSuffix(lines[at+1], "│ first line") {
				t.Fatalf("want the label at the top of the preview column, the text under it:\n%s", ansi.Strip(onFile))
			}
		})
	}
}
