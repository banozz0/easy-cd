package picker

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// xdgDir is $env/ecd, or ~/fallback/ecd when env is unset.
func xdgDir(env, fallback string) string {
	if dir := os.Getenv(env); filepath.IsAbs(dir) {
		return filepath.Join(dir, "ecd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback, "ecd")
}

// stateDir is where the visit log and pins live: $XDG_STATE_HOME/ecd, or
// ~/.local/state/ecd when it is unset.
func stateDir() string { return xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state")) }

func visitsFile() string { return filepath.Join(stateDir(), "visits") }

// recordVisit appends dir and the time to the visit log, unless dir is
// home, / or a temp folder. Landing must never fail on the log, so errors
// are dropped.
func recordVisit(dir string) {
	home, _ := os.UserHomeDir()
	if dir == home || dir == "/" || strings.ContainsRune(dir, '\n') || isTemp(dir, home) {
		return
	}
	if os.MkdirAll(stateDir(), 0o755) != nil {
		return
	}
	f, err := os.OpenFile(visitsFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "%d\t%s\n", time.Now().Unix(), dir)
	f.Close()
	trimVisits()
}

const (
	maxVisits  = 5000 // lines the visit log holds before it is trimmed
	keptVisits = 2500 // its newest lines kept by a trim
)

// trimVisits keeps the newest keptVisits lines of the visit log once it
// holds more than maxVisits, so an open never reads every landing ever made.
func trimVisits() {
	file := visitsFile()
	data, _ := os.ReadFile(file)
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) <= maxVisits+1 { // the last is empty, after the final newline
		return
	}
	replace(file, strings.Join(lines[len(lines)-keptVisits-1:], ""))
}

// staleTemp is the age past which a temp file left by replace is debris: a
// process killed between writing it and the rename.
const staleTemp = 10 * time.Minute

// replace writes data to file through a temp file beside it, named file
// plus a dash and a random suffix, so a reader never sees half of it. It
// first removes stale temp files of file, leaving a fresh one that may be
// another ecd writing right now.
func replace(file, data string) error {
	dir, base := filepath.Split(file)
	list, _ := os.ReadDir(dir)
	for _, e := range list {
		if fi, err := e.Info(); err == nil && strings.HasPrefix(e.Name(), base+"-") && time.Since(fi.ModTime()) > staleTemp {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	tmp, err := os.CreateTemp(dir, base+"-*")
	if err != nil {
		return err
	}
	_, err = tmp.WriteString(data)
	if err := cmp.Or(err, tmp.Close()); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// isTemp reports whether dir is under a temp root. A root that holds home
// itself is skipped, so a home that lives in a temp folder still has
// recents.
func isTemp(dir, home string) bool {
	for _, root := range []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders", "/private/var/folders"} {
		if within(dir, root) && !within(home, root) {
			return true
		}
	}
	return false
}

// fromHome is path relative to home, and whether path is home or under it.
func fromHome(path string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(home, path)
	return rel, err == nil && filepath.IsLocal(rel)
}

// within reports whether path is root or under it.
func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(rel)
}

// frecency scores every logged folder, as in zoxide: a visit weighs 4
// within the hour, 2 within the day, 1/2 within the week, 1/4 after.
func frecency() map[string]float64 {
	data, _ := os.ReadFile(visitsFile())
	now := time.Now()
	score := map[string]float64{}
	for _, line := range strings.Split(string(data), "\n") {
		stamp, dir, ok := strings.Cut(line, "\t")
		secs, err := strconv.ParseInt(stamp, 10, 64)
		if !ok || err != nil {
			continue
		}
		switch age := now.Sub(time.Unix(secs, 0)); {
		case age < time.Hour:
			score[dir] += 4
		case age < 24*time.Hour:
			score[dir] += 2
		case age < 7*24*time.Hour:
			score[dir] += 0.5
		default:
			score[dir] += 0.25
		}
	}
	return score
}

// visited returns the logged folders that still exist, best score first.
func visited(score map[string]float64) []string {
	dirs := slices.SortedFunc(maps.Keys(score), func(a, b string) int {
		return cmp.Or(cmp.Compare(score[b], score[a]), strings.Compare(a, b))
	})
	return slices.DeleteFunc(dirs, func(dir string) bool { return !isDir(dir) })
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func pinsFile() string { return filepath.Join(stateDir(), "pins") }

// loadPins returns the pinned folders that still exist, in pin order.
func loadPins() []string {
	data, _ := os.ReadFile(pinsFile())
	var pins []string
	for _, dir := range strings.Split(string(data), "\n") {
		if dir != "" && isDir(dir) {
			pins = append(pins, dir)
		}
	}
	return pins
}

func savePins(pins []string) error {
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	return replace(pinsFile(), strings.Join(pins, "\n")+"\n")
}
