// Package picker is the ecd folder picker: a Bubble Tea program that walks
// folders with the arrow keys and reports the one picked with Enter.
package picker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	dir     string   // folder being listed
	folders []string // names of its subfolders
	cursor  int
	picked  string // absolute path chosen with Enter, empty on Esc
}

// New returns a picker listing the folders of dir.
func New(dir string) tea.Model {
	return model{}.cd(dir, "")
}

// cd lists dir with the highlight on the folder named focus, or the first.
func (m model) cd(dir, focus string) model {
	m.dir, m.folders = dir, subfolders(dir)
	m.cursor = max(slices.Index(m.folders, focus), 0)
	return m
}

func subfolders(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		} else if e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && fi.IsDir() {
				names = append(names, e.Name())
			}
		}
	}
	return names
}

// highlighted is the absolute path under the cursor, or dir itself when it
// has no subfolders.
func (m model) highlighted() string {
	if len(m.folders) == 0 {
		return m.dir
	}
	return filepath.Join(m.dir, m.folders[m.cursor])
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.folders)-1 {
			m.cursor++
		}
	case tea.KeyRight:
		if len(m.folders) > 0 {
			return m.cd(m.highlighted(), ""), nil
		}
	case tea.KeyLeft:
		if parent := filepath.Dir(m.dir); parent != m.dir {
			return m.cd(parent, filepath.Base(m.dir)), nil
		}
	case tea.KeyEnter:
		m.picked = m.highlighted()
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("  " + breadcrumb(m.dir) + "\n")
	for i, name := range m.folders {
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
		}
		b.WriteString(marker + name + "\n")
	}
	return b.String()
}

// breadcrumb renders dir as "~ › code › projects", with home shown as ~.
func breadcrumb(dir string) string {
	const sep = string(filepath.Separator)
	head, rest := sep, dir
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, dir); err == nil && filepath.IsLocal(rel) {
			head, rest = "~", rel
		}
	}
	parts := []string{head}
	for _, p := range strings.Split(rest, sep) {
		if p != "" && p != "." {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " › ")
}

// Finish writes the picked folder to stdout and returns the exit code: 0 when
// a folder was picked, 1 when the picker was left with Esc.
func Finish(final tea.Model, stdout io.Writer) int {
	m, ok := final.(model)
	if !ok || m.picked == "" {
		return 1
	}
	fmt.Fprintln(stdout, m.picked)
	return 0
}
