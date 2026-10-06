// Command ecd is a terminal folder picker. `ecd init <zsh|bash|fish>` prints
// the shell function that runs the picker and cds to the folder it prints.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/banozz0/easy-cd/internal/picker"
)

// inits are the ecd functions per shell. With arguments each passes them to
// the binary; bare, it runs the picker and cds only when a folder is printed.
// zsh and bash share one text, kept to what macOS's bash 3.2 runs. fish calls
// its cd function rather than the builtin, so cd - and prevd still come back.
var inits = map[string]string{
	"zsh":  shInit,
	"bash": shInit,
	"fish": `function ecd
  if set -q argv[1]; command ecd $argv; return; end
  set -l dir (command ecd); or return
  test -n "$dir"; and cd -- $dir
end
`,
}

const shInit = `ecd() {
  if (( $# )); then command ecd "$@"; return; fi
  local dir
  dir="$(command ecd)" || return
  [[ -n $dir ]] && builtin cd -- "$dir"
}
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 2 && args[0] == "init" && inits[args[1]] != "" {
		fmt.Print(inits[args[1]])
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, `usage: ecd                       pick a folder
       ecd init <zsh|bash|fish>  print the shell function, then add to your shell's config:
                                   zsh   eval "$(ecd init zsh)"   in ~/.zshrc
                                   bash  eval "$(ecd init bash)"  in ~/.bashrc
                                   fish  ecd init fish | source   in ~/.config/fish/config.fish`)
		return 2
	}

	final, err := pick()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ecd:", err)
		return 2
	}
	return picker.Finish(final, os.Stdout)
}

// pick runs the picker in the current folder. It draws on the terminal so
// stdout carries only the picked path.
func pick() (tea.Model, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("needs a terminal: %w", err)
	}
	defer tty.Close()
	// Styles follow the terminal drawn on, not stdout, which the shell captures.
	lipgloss.SetColorProfile(lipgloss.NewRenderer(tty).ColorProfile())
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return tea.NewProgram(picker.New(dir), tea.WithInput(tty), tea.WithOutput(tty)).Run()
}
