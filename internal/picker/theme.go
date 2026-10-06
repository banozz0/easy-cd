package picker

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The one theme, on by default: Catppuccin Mocha's calm colours, as in the
// approved prototype. Lip Gloss brings them down to what the terminal shows,
// so each is rendered when drawn, never once at start.
var (
	blue   = lipgloss.Color("#89b4fa")
	mauve  = lipgloss.Color("#cba6f7")
	yellow = lipgloss.Color("#f9e2af")
	bar    = "#313244" // the highlighted row's background

	dim     = lipgloss.NewStyle().Faint(true)
	hit     = lipgloss.NewStyle().Bold(true).Foreground(yellow) // matched letters
	onPath  = lipgloss.NewStyle().Bold(true)                    // the way down in the columns
	crumb   = lipgloss.NewStyle().Bold(true).Foreground(blue)
	marker  = lipgloss.NewStyle().Foreground(blue) // the highlight's ▸
	chosen  = lipgloss.NewStyle().Bold(true)       // the highlighted row's name
	pinned  = lipgloss.NewStyle().Foreground(yellow)
	recent  = lipgloss.NewStyle().Foreground(mauve)
	branch  = lipgloss.NewStyle().Foreground(mauve)
	unsaved = lipgloss.NewStyle().Foreground(yellow)                    // the dirty dot
	key     = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8")) // a key in the help
)

// Row icons: emoji, which every terminal font draws, unlike Nerd Font glyphs.
const (
	folderIcon = "📁"
	fileIcon   = "📄"
)

// onBar lays row on the selection bar, padded to w cells: the bar's colour
// is set again after every reset inside row, so its styled parts keep it.
func onBar(row string, w int) string {
	seq := lipgloss.ColorProfile().Color(bar).Sequence(true)
	if seq == "" {
		return row
	}
	on := "\x1b[" + seq + "m"
	pad := strings.Repeat(" ", max(w-ansi.StringWidth(row), 0))
	return on + strings.ReplaceAll(row, "\x1b[0m", "\x1b[0m"+on) + pad + "\x1b[0m"
}
