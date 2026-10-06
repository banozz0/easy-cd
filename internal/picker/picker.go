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
	offset  int      // first folder shown when the list is taller than the screen
	height  int      // terminal rows, 0 until the first resize
	pins    []string // pinned folders, at most maxPins
	recent  []string // most-visited folders, best first
	query   string   // search text typed so far
	notice  string   // one-off message shown until the next key
	picked  string   // absolute path landed on, empty on Esc
}

const (
	maxPins  = 3
	numSlots = 9
)

// New returns a picker listing the folders of dir.
func New(dir string) tea.Model {
	// Load enough recents to fill slots 4-9 even when all pins are among them.
	m := model{pins: loadPins(), recent: recents(numSlots)}
	return m.cd(dir, "")
}

// slots are the jump targets for digits 1-9: pins on 1-3, then the recents
// not already pinned on 4-9. An empty string is an empty slot.
func (m model) slots() [numSlots]string {
	var slots [numSlots]string
	copy(slots[:], m.pins)
	i := maxPins
	for _, dir := range m.recent {
		if i < numSlots && !slices.Contains(m.pins, dir) {
			slots[i] = dir
			i++
		}
	}
	return slots
}

// togglePin pins the highlighted folder, or unpins it when already pinned.
// A pin past maxPins is refused with a notice rather than evicting one.
func (m model) togglePin() model {
	dir := m.highlighted()
	pins := slices.Clone(m.pins)
	if i := slices.Index(pins, dir); i >= 0 {
		pins = slices.Delete(pins, i, i+1)
	} else if len(pins) == maxPins {
		m.notice = fmt.Sprintf("%d pins already: unpin one with Ctrl+P first", maxPins)
		return m
	} else {
		pins = append(pins, dir)
	}
	if err := savePins(pins); err != nil {
		m.notice = "pins not saved: " + err.Error()
		return m
	}
	m.pins = pins
	return m
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
		} else if e.Type()&os.ModeSymlink != 0 && isDir(filepath.Join(dir, e.Name())) {
			names = append(names, e.Name())
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
	m, cmd := m.update(msg)
	return m.scrolled(), cmd
}

func (m model) update(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		m.notice = ""
		return m.key(msg)
	}
	return m, nil
}

func (m model) key(key tea.KeyMsg) (model, tea.Cmd) {
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
	case tea.KeyCtrlP:
		return m.togglePin(), nil
	case tea.KeyRunes:
		if r := key.Runes; m.query == "" && len(r) == 1 && '1' <= r[0] && r[0] <= '9' {
			return m.jump(int(r[0] - '1'))
		}
		m.query += string(key.Runes)
	case tea.KeySpace:
		if m.query != "" {
			m.query += " "
		}
	case tea.KeyEsc, tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// jump lands on slot i, or does nothing when the slot is empty.
func (m model) jump(i int) (model, tea.Cmd) {
	if m.picked = m.slots()[i]; m.picked == "" {
		return m, nil
	}
	return m, tea.Quit
}

// scrolled moves the list window just enough to keep the highlight on screen.
func (m model) scrolled() model {
	rows := m.listRows(m.header())
	m.offset = min(m.offset, m.cursor, max(len(m.folders)-rows, 0))
	m.offset = max(m.offset, m.cursor-rows+1)
	return m
}

// listRows is how many folders fit under header.
func (m model) listRows(header string) int {
	if m.height == 0 {
		return len(m.folders)
	}
	return max(m.height-strings.Count(header, "\n"), 1)
}

func (m model) View() string {
	header := m.header()
	var b strings.Builder
	b.WriteString(header)
	end := min(m.offset+m.listRows(header), len(m.folders))
	for i := m.offset; i < end; i++ {
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
		}
		b.WriteString(marker + m.folders[i] + "\n")
	}
	return b.String()
}

// header is the lines above the folder list: breadcrumb, then the search
// text or the jump slots, then any notice.
func (m model) header() string {
	var b strings.Builder
	b.WriteString("  " + breadcrumb(m.dir) + "\n")
	if m.query != "" {
		b.WriteString("  search › " + m.query + "\n")
	} else {
		slots := m.slots()
		b.WriteString(slotRow("★ pinned", slots[:maxPins], 0))
		b.WriteString(slotRow("◷ recent", slots[maxPins:], maxPins))
	}
	if m.notice != "" {
		b.WriteString("  " + m.notice + "\n")
	}
	return b.String()
}

// slotRow renders filled slots as "  ★ pinned  1 code  2 notes", numbered
// from first+1, or nothing when every slot is empty.
func slotRow(label string, slots []string, first int) string {
	row := ""
	for i, dir := range slots {
		if dir != "" {
			row += fmt.Sprintf("  %d %s", first+i+1, filepath.Base(dir))
		}
	}
	if row == "" {
		return ""
	}
	return "  " + label + row + "\n"
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

// Finish writes the picked folder to stdout, logs it as a visit and returns
// the exit code: 0 when a folder was picked, 1 when the picker was left with
// Esc.
func Finish(final tea.Model, stdout io.Writer) int {
	m, ok := final.(model)
	if !ok || m.picked == "" {
		return 1
	}
	recordVisit(m.picked)
	fmt.Fprintln(stdout, m.picked)
	return 0
}
