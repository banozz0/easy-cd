# easy-cd

Stop typing paths. Type `ecd`, pick the folder with your arrow keys, press Enter.

```
~ ❯ ecd
  ~ › code › projects
  ★ pinned  1 projects  2 work  3 notes
  ◷ recent  4 website  5 notes
▸ 📁 my-app  ⎇ main ●  2m
  📁 blog  3d
  ↑↓ move  → in  ← up  ⏎ cd  1-9 jump  type search  tab columns  esc quit
```

- **Your folders first.** Pinned folders and the ones you visit most sit on top; 1–9 jumps.
- **Type to search everywhere.** Exact names first, then names that start or contain it.
- **Tab for columns.** Finder-style: every level stays visible.
- **Files too.** Highlight to preview, → to read the whole thing, Enter opens it in its
  default app.

## Install

```sh
brew install banozz0/tap/easy-cd
```

Or with Go 1.25+: `go install github.com/banozz0/easy-cd/cmd/ecd@latest`. Prebuilt
macOS and Linux binaries are on the [releases page](https://github.com/banozz0/easy-cd/releases).

Then add one line to your shell's config and open a new shell:

| Shell | Line | File |
|---|---|---|
| zsh | `eval "$(ecd init zsh)"` | `~/.zshrc` |
| bash | `eval "$(ecd init bash)"` | `~/.bashrc` (macOS: `~/.bash_profile`) |
| fish | `ecd init fish \| source` | `~/.config/fish/config.fish` |

The line defines an `ecd` function that changes your shell's folder. Plain `cd` is left alone.

## Keys

```
↑ ↓       move
→         open a folder, read a file
←         up one level
Enter     cd there; a file opens in its app and your shell lands in its folder
letters   search every folder under home (Backspace edits, Esc clears)
1–9       jump to a pinned or recent folder
Ctrl+P    pin or unpin the highlighted folder
Shift+←   back to home
Tab       flip between the list and Finder-style columns
Esc       quit; your shell stays where it was
```

In the reader, ↑ ↓ PgUp PgDn scroll and ← or Esc goes back.

## License

MIT
