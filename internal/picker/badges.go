package picker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// age is how long ago path last changed, short and dimmed: now, 3m, 2h,
// 5d, 2y. It is empty when path can't be read.
func age(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return dim.Render(ago(time.Since(fi.ModTime())))
}

// ago is d in its largest whole unit: now, then minutes, hours, days, years.
func ago(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d < day:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d < 365*day:
		return fmt.Sprintf("%dd", d/day)
	}
	return fmt.Sprintf("%dy", d/(365*day))
}

// repo is a folder's git badge: its branch, empty while git hasn't answered
// or when the folder is not a repo, and whether it has unsaved changes.
type repo struct {
	path   string
	branch string
	dirty  bool
}

// gitTimeout bounds one git call, so a huge or wedged repo shows no badge
// instead of holding a process open.
const gitTimeout = 2 * time.Second

// gitStatus asks git for the branch and dirty state of the repo at dir.
// A folder without .git, a git error or a timeout gives no branch.
func gitStatus(dir string) repo {
	r := repo{path: dir}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	// No optional locks: a badge must never take the index lock from a git
	// command the user is running.
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "-C", dir, "status", "--porcelain=v2", "--branch")
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		return r
	}
	var oid string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			oid = strings.TrimPrefix(line, "# branch.oid ")
		case strings.HasPrefix(line, "# branch.head "):
			r.branch = strings.TrimPrefix(line, "# branch.head ")
		case line != "" && !strings.HasPrefix(line, "#"):
			r.dirty = true
		}
	}
	if r.branch == "(detached)" {
		r.branch = oid[:min(7, len(oid))]
	}
	return r
}

// badged asks git about the folders on screen it hasn't asked about yet,
// each in the background, so the list never waits on git. The answers are
// kept for the session.
func (m model) badged() tea.Cmd {
	var asks []tea.Cmd
	if m.height == 0 { // every row counts as on screen until the size lands
		return nil
	}
	for i := range m.shown(m.listRows(m.header(), m.rows())) {
		dir := m.rowPath(i)
		if _, asked := m.repos[dir]; asked {
			continue
		}
		m.repos[dir] = repo{}
		asks = append(asks, func() tea.Msg { return gitStatus(dir) })
	}
	return tea.Batch(asks...)
}

// badge is a repo's branch with a dot when dirty, empty for no repo.
func (r repo) badge() string {
	if r.dirty {
		return r.branch + " ●"
	}
	return r.branch
}
