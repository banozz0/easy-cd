# easy-cd — agent brief

**Private until the v1 release, then public** (MIT). `ecd` is a terminal folder picker: favourites and recent folders on
top, arrow keys to walk, type to search everywhere, Tab for Finder-style columns, Enter to
cd. Go + Charm's Bubble Tea / Lip Gloss, one binary, macOS + Linux, zsh/bash/fish.

- Shaping record and every decision: `easy-cd/IDEA.md` in the incubator repo.
- Spec, dashboard and journal: vault `Projects/Easy CD/`. Read `ecd v1 spec.md` before
  touching code.
- Status: graduated 2026-10-06; walking skeleton built (zsh init, list, arrows, Enter, Esc);
  jump slots built (visit log, frecency recents, Ctrl+P pins, 1–9, list scrolling);
  search built (folder index cached under XDG_CACHE_HOME/ecd, spec ranking, Backspace/Esc);
  columns built (Tab toggles, trail folds past 4 levels, Shift+← home); file preview built
  (files dimmed after folders, first 64 KB read in the background when highlighted, beside the
  list and in the preview column, notice for binary, over 10 MB or non-regular files); file
  reader built (Right reads the whole file, chroma colour under 512 KB, arrows and page keys
  scroll, Left or Esc back; Enter on a file opens it in its default app and the shell lands in
  its folder).
- Code: `cmd/ecd` (binary, shell init), `internal/picker` (Bubble Tea model). Test with
  `go test ./...`; the shell seam needs zsh on PATH.

## Hard rules

- **Never override `cd`.** The command is `ecd`. Agent shells load Sven's profile, and a
  bare `cd` opening an interactive picker would hang them.
- **Letters always search.** Commands live on arrows, Enter, Esc, Tab, Backspace, digits
  1–9 (empty search only), Ctrl+P (pin) and Shift+← (home). Never bind a bare letter.
- **Folder index only.** The cache holds folder names, never file contents; a file is read
  when highlighted, in full only in the reader.
- **Goes public at release:** nothing from Sven's machine in git: no real paths, folder names, visit
  logs or screenshots of his home. Test fixtures build their own temp trees.

## Agent skills

Issue tracker: the shared Beads board (`agent-board` repo), every card tagged
`project:easy-cd`, written per canon `board-writing.md` (agents repo, `src/canon/`).
Domain docs: single-context. Specs, plans and decision records go in the vault project
folder, never in this repo (canon: repos carry no documents).
