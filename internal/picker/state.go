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

// stateDir is where the visit log and pins live: $XDG_STATE_HOME/ecd, or
// ~/.local/state/ecd when it is unset.
func stateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "ecd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "ecd")
}

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
	defer f.Close()
	fmt.Fprintf(f, "%d\t%s\n", time.Now().Unix(), dir)
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

// within reports whether path is root or under it.
func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(rel)
}

// recents returns up to n logged folders that still exist, best score
// first. A visit's weight falls with its age, as in zoxide:
// 4 within the hour, 2 within the day, 1/2 within the week, 1/4 after.
func recents(n int) []string {
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
	dirs := slices.SortedFunc(maps.Keys(score), func(a, b string) int {
		return cmp.Or(cmp.Compare(score[b], score[a]), strings.Compare(a, b))
	})
	var live []string
	for _, dir := range dirs {
		if len(live) == n {
			break
		}
		if isDir(dir) {
			live = append(live, dir)
		}
	}
	return live
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
	return os.WriteFile(pinsFile(), []byte(strings.Join(pins, "\n")+"\n"), 0o644)
}
