package picker

import "github.com/charmbracelet/lipgloss"

// The one theme, on by default: Catppuccin Mocha's calm colours, as in the
// approved prototype. Lip Gloss brings them down to what the terminal shows,
// so each is rendered when drawn, never once at start.
var (
	blue   = lipgloss.Color("#89b4fa")
	mauve  = lipgloss.Color("#cba6f7")
	yellow = lipgloss.Color("#f9e2af")

	dim     = lipgloss.NewStyle().Faint(true)
	hit     = lipgloss.NewStyle().Bold(true).Foreground(yellow) // matched letters
	onPath  = lipgloss.NewStyle().Bold(true)                    // the way down in the columns
	crumb   = lipgloss.NewStyle().Bold(true).Foreground(blue)
	marker  = lipgloss.NewStyle().Foreground(blue) // the highlight's ▸
	chosen  = lipgloss.NewStyle().Bold(true)       // the highlighted row's name
	pinned  = lipgloss.NewStyle().Foreground(yellow)
	recent  = lipgloss.NewStyle().Foreground(mauve)
	branch  = lipgloss.NewStyle().Foreground(mauve)
	unsaved = lipgloss.NewStyle().Foreground(yellow) // the dirty dot
)

// Row icons: emoji, which every terminal font draws, unlike Nerd Font glyphs.
const (
	folderIcon = "📁"
	fileIcon   = "📄"
)
