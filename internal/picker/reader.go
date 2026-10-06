package picker

import (
	"cmp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// reader is the full-screen view of one file. Its path is empty while it
// is closed.
type reader struct {
	file   preview // the whole file, or a notice
	offset int     // first line on screen
}

// reading opens the reader on path, a notice showing until the file is read.
func reading(path string) (reader, tea.Cmd) {
	return reader{file: preview{path: path, notice: "reading…"}}, readAll(path)
}

func (r reader) open() bool { return r.file.path != "" }

// readMsg carries a file read in full for the reader.
type readMsg preview

// readAll reads the whole file at path in the background, so a slow disk
// never stalls the keys.
func readAll(path string) tea.Cmd {
	return func() tea.Msg {
		p := readFile(path, maxFile)
		p.lines = highlight(path, p.lines)
		return readMsg(p)
	}
}

// colourFormatters are the chroma formatters for the colour profiles that have
// colours.
var colourFormatters = map[termenv.Profile]string{
	termenv.ANSI:      "terminal",
	termenv.ANSI256:   "terminal256",
	termenv.TrueColor: "terminal16m",
}

// maxHighlight is the size past which the reader shows a file uncoloured:
// chroma takes about 2 s a megabyte, and the reader should open in about one.
const maxHighlight = 512 << 10

// highlight colours lines as the language of the file at path, guessed from
// its name or else its text, each line coloured on its own so any window of
// them renders right. They come back unchanged on a terminal without colour
// or past maxHighlight.
func highlight(path string, lines []string) []string {
	formatter, ok := colourFormatters[lipgloss.ColorProfile()]
	size := 0
	for _, line := range lines {
		size += len(line) + 1
	}
	if !ok || len(lines) == 0 || size > maxHighlight {
		return lines
	}
	text := strings.Join(lines, "\n")
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = cmp.Or(lexers.Analyse(text), lexers.Fallback)
	}
	tokens, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return lines
	}
	f, style := formatters.Get(formatter), styles.Get("nord")
	coloured := make([]string, len(lines))
	for i, line := range chroma.SplitTokensIntoLines(tokens.Tokens()) {
		var b strings.Builder
		if i == len(lines) || f.Format(&b, style, chroma.Literator(line...)) != nil {
			return lines
		}
		coloured[i] = strings.ReplaceAll(b.String(), "\n", "")
	}
	return coloured
}

// scrolled moves the reader by lines, keeping a full screen of rows in view.
func (r reader) scrolled(by, rows int) reader {
	r.offset = max(min(r.offset+by, len(r.file.lines)-rows), 0)
	return r
}

// readerHeader is the line above the file: its path.
func (m model) readerHeader() string { return "  " + breadcrumb(m.reader.file.path) + "\n" }

// pageRows is how many lines of the file fit under its path.
func (m model) pageRows() int { return m.listRows(m.readerHeader(), len(m.reader.file.lines)) }

// readerKey scrolls the open file. Left and Esc close it, Enter and Ctrl+C
// do what they do on the list, and every other key is ignored.
func (m model) readerKey(key tea.KeyMsg) (model, tea.Cmd) {
	rows := m.pageRows()
	switch key.Type {
	case tea.KeyUp:
		m.reader = m.reader.scrolled(-1, rows)
	case tea.KeyDown:
		m.reader = m.reader.scrolled(1, rows)
	case tea.KeyPgUp:
		m.reader = m.reader.scrolled(-rows, rows)
	case tea.KeyPgDown:
		m.reader = m.reader.scrolled(rows, rows)
	case tea.KeyLeft, tea.KeyEsc:
		m.reader = reader{}
	case tea.KeyEnter, tea.KeyCtrlC:
		return m.key(key)
	}
	return m, nil
}

// readerView is the file's path over as many of its lines as fit from
// offset, or its notice, each cut to the terminal's width.
func (m model) readerView() string {
	rows := m.pageRows()
	r := m.reader.scrolled(0, rows)
	page := preview{lines: r.file.lines[r.offset:], notice: r.file.notice}
	var b strings.Builder
	b.WriteString(m.readerHeader())
	for _, line := range page.cells(rows) {
		b.WriteString("  " + ansi.Truncate(line, cmp.Or(m.width, 80)-2, "…") + "\n")
	}
	return b.String()
}
