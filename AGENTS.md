# easy-cd — agent brief

**Public** (MIT), released as v1.0.0 through the banozz0/homebrew-tap cask. `ecd` is a
terminal folder picker: favourites and recent folders on top, arrow keys to walk, type to
search everywhere, Tab for Finder-style columns, Enter to cd. Go + Charm's Bubble Tea /
Lip Gloss, one binary, macOS + Linux, zsh/bash/fish.

- Shaping record and every decision: `easy-cd/IDEA.md` in the incubator repo.
- Spec, dashboard and journal: vault `Projects/Easy CD/`. Read `ecd v1 spec.md` before
  touching code.
- Status: graduated 2026-10-06; every v1 slice built: walking skeleton (zsh init, arrows,
  Enter, Esc); jump slots (visit log trimmed past 5000 lines, frecency recents, Ctrl+P pins,
  1–9); search (folder index cached under XDG_CACHE_HOME/ecd, spec ranking); columns (Tab
  toggles, or opens them at the highlighted search result; trail folds past 4 levels; Shift+←
  home); file preview (first 64 KB read in the background, notice for binary, over 10 MB or
  non-regular files); file reader (Right; chroma colour under 512 KB; Enter opens a file in its
  default app and lands in its folder); row badges (dimmed age, async git branch and dirty dot);
  bash and fish init; final polish (Catppuccin Mocha theme, emoji icons, selection bar, key
  help, rows cut to the width keeping badges, warm open under 100 ms tested through a pty).
  Release: goreleaser run locally (.goreleaser.yaml), never GitHub Actions.
- Code: `cmd/ecd` (binary, shell init), `internal/picker` (Bubble Tea model). Test with
  `go test ./...`; the shell seam runs zsh, /bin/bash and fish.

## Hard rules

- **Never override `cd`.** The command is `ecd`. Agent shells load Sven's profile, and a
  bare `cd` opening an interactive picker would hang them.
- **Letters always search.** Commands live on arrows, Enter, Esc, Tab, Backspace, digits
  1–9 (empty search only), Ctrl+P (pin) and Shift+← (home). Never bind a bare letter.
- **Folder index only.** The cache holds folder names, never file contents; a file is read
  when highlighted, in full only in the reader.
- **Public repo:** nothing from Sven's machine in git: no real paths, folder names, visit
  logs or screenshots of his home. Test fixtures build their own temp trees.

## Agent skills

Issue tracker: the shared Beads board (`agent-board` repo), every card tagged
`project:easy-cd`, written per canon `board-writing.md` (agents repo, `src/canon/`).
Domain docs: single-context. Specs, plans and decision records go in the vault project
folder, never in this repo (canon: repos carry no documents).
