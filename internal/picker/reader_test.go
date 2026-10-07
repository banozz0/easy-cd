package picker_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

var (
	pgDown = tea.KeyMsg{Type: tea.KeyPgDown}
	ctrlC  = tea.KeyMsg{Type: tea.KeyCtrlC}
)

// waitAfter waits until the terminal has shown want after the last time it
// showed mark, so text the list showed before mark doesn't count.
func waitAfter(t *testing.T, tm *teatest.TestModel, mark, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		i := bytes.LastIndex(out, []byte(mark))
		return i >= 0 && bytes.Contains(out[i:], []byte(want))
	}, teatest.WithDuration(5*time.Second))
}

// inReader starts the picker in dir, opens the reader on the file
// highlighted first, waits until it shows want under the file's path mark,
// quits there and returns the reader's last screen.
func inReader(t *testing.T, dir, mark, want string) string {
	t.Helper()
	tm := newPicker(t, dir)
	tm.Send(right)
	waitAfter(t, tm, mark, want)
	tm.Send(ctrlC)
	return tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()
}

func TestReaderScrollsByPageAndLeftReturnsToTheSameHighlight(t *testing.T) {
	home := newHome(t, "docs")
	docs := filepath.Join(home, "docs")
	var text strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&text, "line %03d\n", i)
	}
	writeFile(t, docs, "a.txt", "a\n")
	writeFile(t, docs, "notes.txt", text.String())
	writeFile(t, docs, "z.txt", "z\n")
	tm := newPicker(t, docs)

	tm.Send(down) // onto notes.txt
	tm.Send(right)
	waitAfter(t, tm, "docs › notes.txt", "line 010")
	tm.Send(pgDown)
	waitFor(t, tm, "line 040") // past the first 24-row screen
	tm.Send(left)
	tm.Send(esc)

	screen := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()
	if !strings.Contains(screen, "▸ 📄 notes.txt") || strings.Contains(screen, "line 040") {
		t.Fatalf("want the list back with notes.txt highlighted:\n%s", screen)
	}
}

func TestReaderShowsANoticeForABinaryFile(t *testing.T) {
	home := newHome(t, "bin")
	writeFile(t, filepath.Join(home, "bin"), "blob", "MAGIC\x00\x01\x02PAYLOAD\n")

	screen := inReader(t, filepath.Join(home, "bin"), "bin › blob", "binary file")

	if !strings.Contains(screen, "bin › blob") || strings.Contains(screen, "MAGIC") || strings.Contains(screen, "PAYLOAD") {
		t.Fatalf("want the reader on blob showing only the notice:\n%s", screen)
	}
}

// stubOpener points ECD_OPENER at a script that records the path it is
// asked to open, and returns the record's path.
func stubOpener(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "opened")
	t.Setenv("ECD_OPENER", writeScript(t, dir, "open", "printf '%s' \"$1\" > '"+record+"'\n"))
	return record
}

func TestEnterOpensTheFileAndLandsInItsFolder(t *testing.T) {
	for name, keys := range map[string][]tea.KeyMsg{
		"from the list":   {enter},
		"from the reader": {right, enter},
	} {
		t.Run(name, func(t *testing.T) {
			home := newHome(t, "docs")
			docs := filepath.Join(home, "docs")
			writeFile(t, docs, "plan.txt", "x\n")
			record := stubOpener(t)

			out, code := run(t, docs, keys...)

			opened, _ := os.ReadFile(record)
			if want := filepath.Join(docs, "plan.txt"); string(opened) != want {
				t.Fatalf("opener got %q, want %q", opened, want)
			}
			if out != docs+"\n" || code != 0 {
				t.Fatalf("stdout %q exit %d, want %q exit 0", out, code, docs)
			}
		})
	}
}

func TestReaderColoursSourceByItsLanguage(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	home := newHome(t, "src")
	writeFile(t, filepath.Join(home, "src"), "main.go", "package main\n\nfunc main() {}\n")

	screen := inReader(t, filepath.Join(home, "src"), "src › main.go", "func")

	// The keywords package and func each start their own colour.
	if !strings.Contains(screen, "mpackage") || !strings.Contains(screen, "mfunc") {
		t.Fatalf("want package and func coloured: %q", screen)
	}
}

func TestReaderDropsTerminalControlCodes(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "evil.txt", "safe\x1b]2;pwned\x07 text\r\n")

	screen := inReader(t, filepath.Join(home, "docs"), "docs › evil.txt", "text")

	if !strings.Contains(screen, "text") || strings.ContainsAny(screen, "\x1b\x07\r") {
		t.Fatalf("control codes reached the reader: %q", screen)
	}
}

func TestReaderShowsALargeSourceFileUncoloured(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	home := newHome(t, "src")
	// Over half a megabyte, which would take chroma about a second to colour.
	writeFile(t, filepath.Join(home, "src"), "big.go", "package main\n\n"+strings.Repeat("// filler\n", 60_000))

	screen := inReader(t, filepath.Join(home, "src"), "src › big.go", "filler")

	if !strings.Contains(screen, "  package main\n") {
		t.Fatalf("want package main uncoloured: %q", screen)
	}
}

func TestReaderShowsNoPreviewLabel(t *testing.T) {
	home := newHome(t, "docs")
	writeFile(t, filepath.Join(home, "docs"), "plan.txt", "first line\n")

	screen := inReader(t, filepath.Join(home, "docs"), "docs › plan.txt", "first line")

	if strings.Contains(screen, "preview") {
		t.Fatalf("the reader shows the preview label:\n%s", screen)
	}
}
