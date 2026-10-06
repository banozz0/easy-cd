// Command ecd is a terminal folder picker. `ecd init zsh` prints the shell
// function that runs the picker and cds to the folder it prints.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/banozz0/easy-cd/internal/picker"
)

// zshInit defines the ecd function. With arguments it passes them to the
// binary; bare, it runs the picker and cds only when a folder is printed.
const zshInit = `ecd() {
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
	if len(args) == 2 && args[0] == "init" && args[1] == "zsh" {
		fmt.Print(zshInit)
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, `usage: ecd            pick a folder
       ecd init zsh   print the shell function; add eval "$(ecd init zsh)" to ~/.zshrc`)
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
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return tea.NewProgram(picker.New(dir), tea.WithInput(tty), tea.WithOutput(tty)).Run()
}
