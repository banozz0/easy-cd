package picker_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest"
)

// row is the screen line that lists want, or empty when none does.
func row(screen, want string) string {
	for _, line := range strings.Split(screen, "\n") {
		if name(line) == want {
			return line
		}
	}
	return ""
}

func TestEveryRowShowsHowLongAgoItChanged(t *testing.T) {
	home := newHome(t, "old", "new")
	ago := time.Now().Add(-49 * time.Hour)
	if err := os.Chtimes(filepath.Join(home, "old"), ago, ago); err != nil {
		t.Fatal(err)
	}
	tm := newPicker(t, home)

	waitFor(t, tm, "old")
	tm.Send(esc)
	screen := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()

	if got := strings.Fields(row(screen, "old")); len(got) == 0 || got[len(got)-1] != "2d" {
		t.Fatalf("old row %q, want it to end in 2d, screen:\n%s", row(screen, "old"), screen)
	}
	if got := strings.Fields(row(screen, "new")); len(got) == 0 || got[len(got)-1] != "now" {
		t.Fatalf("new row %q, want it to end in now, screen:\n%s", row(screen, "new"), screen)
	}
}

// gitInit makes dir a git repo on branch, or skips the test without git.
func gitInit(t *testing.T, dir, branch string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if out, err := exec.Command("git", "init", "-q", "-b", branch, dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func TestRepoRowsShowTheirBranchAndADotForUnsavedChanges(t *testing.T) {
	home := newHome(t, "dirty", "clean")
	gitInit(t, filepath.Join(home, "dirty"), "feature-x")
	gitInit(t, filepath.Join(home, "clean"), "main")
	writeFile(t, filepath.Join(home, "dirty"), "draft.txt", "unsaved\n")
	tm := newPicker(t, home)

	waitFor(t, tm, "feature-x", "main")
	tm.Send(esc)
	screen := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()

	if dirty := row(screen, "dirty"); !strings.Contains(dirty, "feature-x ●") {
		t.Fatalf("dirty row %q, want feature-x and the dot, screen:\n%s", dirty, screen)
	}
	if clean := row(screen, "clean"); !strings.Contains(clean, "main") || strings.Contains(clean, "●") {
		t.Fatalf("clean row %q, want main and no dot, screen:\n%s", clean, screen)
	}
}

func TestListShowsAndMovesBeforeGitAnswersAndBadgesFillInAfter(t *testing.T) {
	home := newHome(t, "repo", "zz")
	gitInit(t, filepath.Join(home, "repo"), "feature-x")
	real, _ := exec.LookPath("git")
	bin := t.TempDir()
	writeScript(t, bin, "git", "sleep 1\nexec "+real+" \"$@\"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	tm := newPicker(t, home)

	// The first frame and the first key both land while git still sleeps.
	for _, want := range []string{"repo", "▸ 📁 zz"} {
		var seen []byte
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			seen = out
			return bytes.Contains(out, []byte(want))
		}, teatest.WithDuration(5*time.Second))
		if bytes.Contains(seen, []byte("feature-x")) {
			t.Fatalf("branch on screen before %q, so the list waited on git:\n%s", want, seen)
		}
		tm.Send(down)
	}
	waitFor(t, tm, "feature-x")
	tm.Send(esc)
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestNarrowListCutsRowsToTheWidthAndKeepsTheBadge(t *testing.T) {
	home := newHome(t, "aaaa-long-folder/bbbb-long-folder/cccc-long-folder/the-repo")
	gitInit(t, filepath.Join(home, "aaaa-long-folder/bbbb-long-folder/cccc-long-folder/the-repo"), "feature-x")
	tm := newPicker(t, home, teatest.WithInitialTermSize(40, 24))
	for _, k := range typed("the-repo") {
		tm.Send(k)
	}
	waitFor(t, tm, "feature-x")
	tm.Send(enter)

	screen := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).View()
	for _, line := range strings.Split(screen, "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Fatalf("line %d wide on a 40-column terminal: %q", w, line)
		}
	}
	// The path is shortened from the left: the folder's name and the badge stay.
	repo := func(r string) bool { return strings.HasSuffix(r, "/the-repo") }
	if !slices.ContainsFunc(rows(screen), repo) || !strings.Contains(screen, "feature-x") {
		t.Fatalf("result row lost its name, or the branch badge was cut, screen:\n%s", screen)
	}
}
