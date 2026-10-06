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
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	dir     string   // folder being listed
	folders []string // names of its subfolders
	files   []string // names of its other entries, listed after the folders
	cursor  int
	offset  int      // first folder shown when the list is taller than the screen
	height  int      // terminal rows, 0 until the first resize
	width   int      // terminal columns, 0 until the first resize
	columns bool     // Finder-style columns instead of the list
	pins    []string // pinned folders, at most maxPins
	query   string   // search text typed so far
	results []match  // folders matching query, best first
	logged  []string // visited folders that still exist, best score first
	index   []string // folders searched: the folder index plus logged
	indexed bool     // whether the background index refresh has landed
	refresh *index
	bonus   map[string]int // search bonus of visited folders
	notice  string         // one-off message shown until the next key
	preview preview        // the highlighted file's start, read when highlighted
	picked  string         // absolute path landed on, empty on Esc
}

const (
	maxPins  = 3
	numSlots = 9
)

// New returns a picker listing the folders of dir.
func New(dir string) tea.Model {
	score := frecency()
	m := model{pins: loadPins(), logged: visited(score), bonus: map[string]int{}}
	for dir, s := range score {
		m.bonus[dir] = min(int(10*s), maxBonus)
	}
	// Logged folders are searchable however deep the index reaches.
	m.index, m.refresh = loadIndex(m.logged)
	return m.cd(dir, "")
}

// maxBonus caps how far visits lift a search result: past a deeper match,
// never past a better kind of match.
const maxBonus = 40

// union is a followed by the paths of b it lacks.
func union(a, b []string) []string {
	seen := map[string]bool{}
	var all []string
	for _, p := range slices.Concat(a, b) {
		if !seen[p] {
			seen[p] = true
			all = append(all, p)
		}
	}
	return all
}

// indexMsg reports that the background index refresh has finished.
type indexMsg struct{}

// slots are the jump targets for digits 1-9: pins on 1-3, then the recents
// not already pinned on 4-9. An empty string is an empty slot.
func (m model) slots() [numSlots]string {
	var slots [numSlots]string
	copy(slots[:], m.pins)
	i := maxPins
	for _, dir := range m.logged {
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
	dir := m.folder()
	if dir == "" {
		return m
	}
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
	m.dir = dir
	m.folders, m.files = entries(dir)
	m.cursor = max(slices.Index(m.folders, focus), 0)
	return m
}

// entries are the names in dir: its subfolders, symlinks to folders
// included, and everything else as files. Nothing is read but the names.
func entries(dir string) (folders, files []string) {
	list, _ := os.ReadDir(dir)
	for _, e := range list {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 && isDir(filepath.Join(dir, e.Name())) {
			folders = append(folders, e.Name())
		} else {
			files = append(files, e.Name())
		}
	}
	return folders, files
}

// highlighted is the absolute path under the cursor: a search result, or
// an entry of dir, or dir itself when it has none. It is empty when a
// search matches nothing.
func (m model) highlighted() string {
	if m.query != "" {
		if len(m.results) == 0 {
			return ""
		}
		return m.results[m.cursor].path
	}
	if m.rows() == 0 {
		return m.dir
	}
	return filepath.Join(m.dir, m.entry(m.cursor))
}

// entry is the name on row i of dir's list: folders first, then files.
func (m model) entry(i int) string {
	if i < len(m.folders) {
		return m.folders[i]
	}
	return m.files[i-len(m.folders)]
}

// file is the absolute path of the highlighted file, or empty when the
// highlight is not on a file.
func (m model) file() string {
	if m.query != "" || m.cursor < len(m.folders) || m.cursor >= m.rows() {
		return ""
	}
	return filepath.Join(m.dir, m.files[m.cursor-len(m.folders)])
}

// folder is highlighted unless the highlight is on a file, which the
// folder keys leave alone.
func (m model) folder() string {
	if m.file() != "" {
		return ""
	}
	return m.highlighted()
}

// rows is how many entries the list holds: results while searching,
// folders and files otherwise.
func (m model) rows() int {
	if m.query != "" {
		return len(m.results)
	}
	return len(m.folders) + len(m.files)
}

// searched sets the query and reruns the search, highlight on the best.
func (m model) searched(query string) model {
	m.query, m.cursor, m.results = query, 0, nil
	if query != "" {
		m.results = search(query, m.dir, m.index, m.bonus)
	}
	return m
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		<-m.refresh.done
		return indexMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := m.update(msg)
	m, read := m.scrolled().previewed()
	return m, tea.Batch(cmd, read)
}

// previewed starts reading the highlighted file in the background when the
// highlight has just landed on it, so a slow disk never stalls the keys,
// and drops the preview when the highlight is off files.
func (m model) previewed() (model, tea.Cmd) {
	file := m.file()
	if file == m.preview.path {
		return m, nil
	}
	m.preview = preview{path: file}
	if file == "" {
		return m, nil
	}
	return m, func() tea.Msg { return peek(file) }
}

func (m model) update(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height, m.width = msg.Height, msg.Width
	case preview:
		if msg.path == m.preview.path { // the highlight may have moved on
			m.preview = msg
		}
	case indexMsg:
		m.index, m.indexed = m.refresh.folders, true
		if m.query != "" {
			was := m.highlighted()
			m = m.searched(m.query)
			m.cursor = max(slices.IndexFunc(m.results, func(r match) bool { return r.path == was }), 0)
		}
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
		if m.cursor < m.rows()-1 {
			m.cursor++
		}
	case tea.KeyRight:
		if dir := m.folder(); m.rows() > 0 && dir != "" {
			return m.searched("").cd(dir, ""), nil
		}
	case tea.KeyLeft:
		if parent := filepath.Dir(m.dir); m.query == "" && parent != m.dir {
			return m.cd(parent, filepath.Base(m.dir)), nil
		}
	case tea.KeyShiftLeft:
		home, _ := os.UserHomeDir()
		return m.searched("").cd(home, ""), nil
	case tea.KeyTab:
		m.columns = !m.columns
	case tea.KeyEnter:
		if m.picked = m.folder(); m.picked != "" { // opening a file is the reader's
			return m, tea.Quit
		}
	case tea.KeyCtrlP:
		return m.togglePin(), nil
	case tea.KeyRunes:
		if r := key.Runes; m.query == "" && len(r) == 1 && '1' <= r[0] && r[0] <= '9' {
			return m.jump(int(r[0] - '1'))
		}
		return m.searched(m.query + string(key.Runes)), nil
	case tea.KeySpace:
		if m.query != "" {
			return m.searched(m.query + " "), nil
		}
	case tea.KeyBackspace:
		if q := []rune(m.query); len(q) > 0 {
			return m.searched(string(q[:len(q)-1])), nil
		}
	case tea.KeyEsc:
		if m.query != "" {
			return m.searched(""), nil
		}
		return m, tea.Quit
	case tea.KeyCtrlC:
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
	m.offset = inView(m.offset, m.cursor, m.listRows(m.header()), m.rows())
	return m
}

// inView is the first of n entries shown rows at a time, moved from offset
// just enough to keep the entry at cursor on screen.
func inView(offset, cursor, rows, n int) int {
	return max(min(offset, cursor, max(n-rows, 0)), cursor-rows+1, 0)
}

// listRows is how many folders fit under header. The view ends in a newline,
// and the empty line after it takes the terminal's last row.
func (m model) listRows(header string) int {
	if m.height == 0 {
		return m.rows()
	}
	return max(m.height-strings.Count(header, "\n")-1, 1)
}

func (m model) View() string {
	header := m.header()
	var b strings.Builder
	b.WriteString(header)
	if m.query != "" && len(m.results) == 0 {
		if m.indexed {
			b.WriteString("  no match\n")
		} else {
			b.WriteString("  indexing…\n")
		}
	}
	if m.columns && m.query == "" {
		b.WriteString(m.columnsView(m.listRows(header)))
		return b.String()
	}
	rows := m.listRows(header)
	if m.preview.path != "" {
		b.WriteString(sideBySide([]column{{m.window(rows), 2}, {m.preview.cells(rows), 3}}, m.width))
		return b.String()
	}
	for _, row := range m.window(rows) {
		b.WriteString(row + "\n")
	}
	return b.String()
}

// window is the list rows on screen, rows at most from offset, the
// highlighted one marked: search results while searching, folders then
// dimmed files otherwise.
func (m model) window(rows int) []string {
	var lines []string
	for i := m.offset; i < min(m.offset+rows, m.rows()); i++ {
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
		}
		if m.query != "" {
			lines = append(lines, marker+m.results[i].render())
			continue
		}
		name := m.entry(i)
		if i >= len(m.folders) {
			name = dim.Render(name)
		}
		lines = append(lines, marker+name)
	}
	return lines
}

var (
	dim = lipgloss.NewStyle().Faint(true)
	hit = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
)

// render shows the result as its path from home, parent dimmed and the
// matched letters of its name highlighted.
func (r match) render() string {
	parent, name := filepath.Split(r.path)
	if rel, ok := fromHome(parent); ok {
		parent = filepath.Join("~", rel) + string(filepath.Separator)
	}
	return dim.Render(parent) + lipgloss.StyleRunes(name, r.hits, hit, lipgloss.NewStyle())
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
	p := path(dir)
	parts := []string{string(filepath.Separator)}
	if _, ok := fromHome(dir); ok {
		parts[0] = "~"
	}
	for _, d := range p[1:] {
		parts = append(parts, filepath.Base(d))
	}
	return strings.Join(parts, " › ")
}

// Finish writes the picked folder to stdout, logs it as a visit and returns
// the exit code: 0 when a folder was picked, 1 when the picker was left with
// Esc. A refresh still scanning dies with the process; the next open
// refreshes again.
func Finish(final tea.Model, stdout io.Writer) int {
	m, ok := final.(model)
	if !ok || m.picked == "" {
		return 1
	}
	recordVisit(m.picked)
	fmt.Fprintln(stdout, m.picked)
	return 0
}
