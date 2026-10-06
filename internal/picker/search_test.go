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

	"github.com/charmbracelet/x/exp/teatest"

	"github.com/banozz0/easy-cd/internal/picker"
)

// searched starts the picker in dir, types query, waits until the terminal
// shows want, and presses Enter. It returns the last screen and what ecd
// printed on stdout.
func searched(t *testing.T, dir, query, want string) (screen, stdout string) {
	t.Helper()
	tm := teatest.NewTestModel(t, picker.New(dir), teatest.WithInitialTermSize(80, 24))
	for _, k := range typed(query) {
		tm.Send(k)
	}
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte(want))
	}, teatest.WithDuration(5*time.Second))
	tm.Send(enter)
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
	tm := teatest.NewTestModel(t, picker.New(home), teatest.WithInitialTermSize(80, 24))
	shows := func(want string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte(want))
		}, teatest.WithDuration(5*time.Second))
	}

	for _, k := range typed("ax") {
		tm.Send(k)
	}
	shows("no match")
	tm.Send(backspace)
	shows("~/beta") // "a" matches both folders again
	tm.Send(esc)
	shows("▸ alpha") // the folder list is back
	tm.Send(esc)

	var stdout bytes.Buffer
	code := picker.Finish(tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)), &stdout)
	if stdout.Len() != 0 || code == 0 {
		t.Fatalf("stdout %q exit %d, want empty and non-zero", stdout.String(), code)
	}
}

func TestCacheHoldsFolderPathsOnly(t *testing.T) {
	home := newHome(t, "code/app", "notes")
	if err := os.WriteFile(filepath.Join(home, "notes", "todo.txt"), []byte("buy milk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run(t, home, esc)

	cache, err := os.ReadFile(filepath.Join(filepath.Dir(home), "cache", "ecd", "folders"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSpace(string(cache)), "\n") {
		if fi, err := os.Stat(line); err != nil || !fi.IsDir() || !filepath.IsAbs(line) {
			t.Errorf("cache line %q is not a folder path", line)
		}
		got = append(got, strings.TrimPrefix(line, home+"/"))
	}
	slices.Sort(got)
	if want := []string{"code", "code/app", "notes"}; !slices.Equal(got, want) {
		t.Fatalf("cache holds %q, want %q", got, want)
	}
}
