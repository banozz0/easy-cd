package picker_test

import (
	"bytes"
	"fmt"
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

	"github.com/banozz0/easy-cd/internal/picker"
)

// searched starts the picker in dir, types query, waits until the terminal
// shows want, then presses keys and Enter. It returns the last screen and
// what ecd printed on stdout.
func searched(t *testing.T, dir, query, want string, keys ...tea.KeyMsg) (screen, stdout string) {
	t.Helper()
	tm := newPicker(t, dir)
	for _, k := range typed(query) {
		tm.Send(k)
	}
	waitFor(t, tm, want)
	for _, k := range append(keys, enter) {
		tm.Send(k)
	}
	final := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
	var out bytes.Buffer
	picker.Finish(final, &out)
	return final.View(), out.String()
}

// rows are the screen's lines with the highlight marker and indent removed.
func rows(screen string) []string {
	lines := strings.Split(screen, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeft(l, "▸ ")
	}
	return lines
}

func TestSearchRanksExactNameFirstThenStartsWithThenContains(t *testing.T) {
	home := newHome(t, "code", "codex", "my-code", "c-o-d-e-ish")

	screen, out := searched(t, home, "code", "~/my-code")

	r := rows(screen)
	exact, starts, contains := slices.Index(r, "~/code"), slices.Index(r, "~/codex"), slices.Index(r, "~/my-code")
	if exact < 0 || !(exact < starts && starts < contains) {
		t.Fatalf("want ~/code above ~/codex above ~/my-code, screen:\n%s", screen)
	}
	if want := filepath.Join(home, "code") + "\n"; out != want {
		t.Fatalf("stdout %q, want %q", out, want)
	}
}

func TestLooseMatchesShowOnlyWhenFewerThanFiveRealOnes(t *testing.T) {
	for _, tc := range []struct {
		real  int
		shown bool
	}{{4, true}, {5, false}} {
		t.Run(fmt.Sprintf("%d real", tc.real), func(t *testing.T) {
			dirs := []string{"axbxc"} // a, b, c spread over 5 letters: tight for abc
			for i := range tc.real {
				dirs = append(dirs, fmt.Sprintf("abc%d", i))
			}
			home := newHome(t, dirs...)

			screen, _ := searched(t, home, "abc", "~/abc0")

			if shown := slices.Contains(rows(screen), "~/axbxc"); shown != tc.shown {
				t.Fatalf("~/axbxc shown %v, want %v, screen:\n%s", shown, tc.shown, screen)
			}
		})
	}
}

func TestSearchSkipsJunkButFindsLoggedFoldersAtAnyDepth(t *testing.T) {
	home := newHome(t, "pub/deep1", "node_modules/deepnm", ".hidden/deephid", "a/b/c/deep4/deep5/deep6")
	run(t, filepath.Join(home, "a/b/c/deep4/deep5/deep6"), enter) // logs a visit 6 levels down

	screen, _ := searched(t, home, "deep", "~/a/b/c/deep4/deep5/deep6")

	r := rows(screen)
	for _, want := range []string{"~/pub/deep1", "~/a/b/c/deep4", "~/a/b/c/deep4/deep5/deep6"} {
		if !slices.Contains(r, want) {
			t.Errorf("%s missing", want)
		}
	}
	for _, never := range []string{"~/node_modules/deepnm", "~/.hidden/deephid", "~/a/b/c/deep4/deep5"} {
		if slices.Contains(r, never) {
			t.Errorf("%s shown", never)
		}
	}
	if t.Failed() {
		t.Logf("screen:\n%s", screen)
	}
}

func TestBackspaceEditsTheSearchAndEscClearsItBeforeQuitting(t *testing.T) {
	home := newHome(t, "alpha", "beta")
	tm := newPicker(t, home)

	for _, k := range typed("ax") {
		tm.Send(k)
	}
	waitFor(t, tm, "no match")
	tm.Send(backspace)
	waitFor(t, tm, "~/beta") // "a" matches both folders again
	tm.Send(esc)
	waitFor(t, tm, "▸ alpha") // the folder list is back
	tm.Send(esc)

	var stdout bytes.Buffer
	code := picker.Finish(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)), &stdout)
	if stdout.Len() != 0 || code == 0 {
		t.Fatalf("stdout %q exit %d, want empty and non-zero", stdout.String(), code)
	}
}

func TestCacheHoldsFolderPathsOnly(t *testing.T) {
	home := newHome(t, "code/app", "notes")
	writeFile(t, filepath.Join(home, "notes"), "todo.txt", "buy milk\n")

	// Previewing todo.txt reads it; none of it may reach the cache.
	previewed(t, filepath.Join(home, "notes"), "buy milk")

	// The refresh writes the cache in the background; it lands whole.
	file := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "ecd", "folders")
	cache, err := os.ReadFile(file)
	for deadline := time.Now().Add(5 * time.Second); os.IsNotExist(err) && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		cache, err = os.ReadFile(file)
	}
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSpace(string(cache)), "\n") {
		if fi, err := os.Stat(line); err != nil || !fi.IsDir() || !filepath.IsAbs(line) || strings.Contains(line, "milk") {
			t.Errorf("cache line %q is not a folder path", line)
		}
		got = append(got, strings.TrimPrefix(line, home+"/"))
	}
	slices.Sort(got)
	if want := []string{"code", "code/app", "notes"}; !slices.Equal(got, want) {
		t.Fatalf("cache holds %q, want %q", got, want)
	}
}

func TestSearchRanksTheCurrentFolderFirstThenShallowerFolders(t *testing.T) {
	home := newHome(t, "x/proj", "x/b/c/proj", "y/proj", "y/a/proj")

	screen, _ := searched(t, filepath.Join(home, "y"), "proj", "~/x/b/c/proj")

	r := rows(screen)
	var at []int
	for _, row := range []string{"~/y/proj", "~/y/a/proj", "~/x/proj", "~/x/b/c/proj"} {
		at = append(at, slices.Index(r, row))
	}
	if at[0] < 0 || !slices.IsSorted(at) {
		t.Fatalf("want ~/y/proj, ~/y/a/proj, ~/x/proj, ~/x/b/c/proj in that order, screen:\n%s", screen)
	}
}

func TestVisitsLiftASearchResult(t *testing.T) {
	home := newHome(t, "p/code", "q/code")
	run(t, filepath.Join(home, "q", "code"), enter)

	_, out := searched(t, home, "code", "~/p/code")

	if want := filepath.Join(home, "q", "code") + "\n"; out != want {
		t.Fatalf("Enter on the top result printed %q, want the visited %q", out, want)
	}
}

func TestRightBrowsesIntoASearchResult(t *testing.T) {
	home := newHome(t, "q/code/inner")

	_, out := searched(t, home, "code", "~/q/code", right)

	if want := filepath.Join(home, "q", "code", "inner") + "\n"; out != want {
		t.Fatalf("stdout %q, want %q", out, want)
	}
}

func TestSearchHighlightsMatchedLettersAndDimsTheParent(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	home := newHome(t, "work/my-code")

	screen, _ := searched(t, home, "code", "my-")

	// faint ~/work/, plain my-, bold blue code
	if want := "\x1b[2m~/work/\x1b[0mmy-\x1b[1;34mcode\x1b[0m"; !strings.Contains(screen, want) {
		t.Fatalf("screen lacks %q:\n%q", want, screen)
	}
}
