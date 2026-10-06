# easy-cd — agent brief

**Private until the v1 release, then public** (MIT). `ecd` is a terminal folder picker: favourites and recent folders on
top, arrow keys to walk, type to search everywhere, Tab for Finder-style columns, Enter to
cd. Go + Charm's Bubble Tea / Lip Gloss, one binary, macOS + Linux, zsh/bash/fish.

- Shaping record and every decision: `easy-cd/IDEA.md` in the incubator repo.
- Spec, dashboard and journal: vault `Projects/Easy CD/`. Read `ecd v1 spec.md` before
  touching code.
- Status: graduated 2026-10-06; walking skeleton built (zsh init, list, arrows, Enter, Esc).
- Code: `cmd/ecd` (binary, shell init), `internal/picker` (Bubble Tea model). Test with
  `go test ./...`; the shell seam needs zsh on PATH.

## Hard rules

- **Never override `cd`.** The command is `ecd`. Agent shells load Sven's profile, and a
  bare `cd` opening an interactive picker would hang them.
- **Letters always search.** Commands live on arrows, Enter, Esc, Tab, digits 1–9 (empty
  search only), Ctrl+P (pin) and Shift+← (home). Never bind a bare letter.
- **Folder index only.** The cache holds folder names, never file contents; a file is read
  when highlighted, in full only in the reader.
- **Goes public at release:** nothing from Sven's machine in git: no real paths, folder names, visit
  logs or screenshots of his home. Test fixtures build their own temp trees.

## Agent skills

Issue tracker: the shared Beads board (`agent-board` repo), every card tagged
`project:easy-cd`, written per canon `board-writing.md` (agents repo, `src/canon/`).
Domain docs: single-context. Specs, plans and decision records go in the vault project
folder, never in this repo (canon: repos carry no documents).
