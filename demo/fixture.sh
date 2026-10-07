#!/bin/sh
# fixture.sh DIR builds the world the demo is recorded in, deleting DIR
# first: DIR/home is a fake home with generic folders, two git repos and a
# few files to preview; DIR/state holds a visit log and pins so the jump
# slots fill; DIR/zsh holds the demo shell's .zshrc. Outside home, so the
# picker's home listing shows only the demo folders.
set -eu
dir=${1:?usage: fixture.sh DIR}
if [ -e "$dir" ] && [ ! -e "$dir/.ecd-demo" ]; then
  echo "fixture.sh: $dir exists and is not a demo fixture; not deleting it" >&2
  exit 1
fi
rm -rf "$dir"
home=$dir/home
mkdir -p "$home"
touch "$dir/.ecd-demo"
cd "$home"

mkdir -p projects/website/src/components projects/website/public \
  projects/api/cmd/server projects/api/internal/store projects/api/internal/handlers \
  projects/scripts notes photos/2025 photos/2026 downloads documents music

cat > projects/website/README.md <<'MD'
# website

The marketing site. Static pages, one small script.

## Run it

    npm install
    npm run dev

Pages live in src/, images in public/.
MD
cat > projects/website/src/app.js <<'JS'
// Greets the visitor by the time of day.
const hour = new Date().getHours();
const greeting = hour < 12 ? "Good morning" : hour < 18 ? "Good afternoon" : "Good evening";

export function greet(name) {
  return `${greeting}, ${name}!`;
}

document.querySelector("#hello").textContent = greet("friend");
JS
cat > projects/website/src/router.js <<'JS'
// A tiny client-side router: maps paths to pages and renders them.
import { header } from "./components/header.js";

const routes = {
  "/": () => page("Home", "Welcome to the website."),
  "/about": () => page("About", "A small team making small tools."),
  "/blog": () => page("Blog", "Notes on building things."),
  "/contact": () => page("Contact", "Write to hello@example.com."),
};

function page(title, text) {
  return `${header(title)}<main><p>${text}</p></main>`;
}

function notFound(path) {
  return page("Not found", `Nothing lives at ${path}.`);
}

export function render(path) {
  const view = routes[path] ?? (() => notFound(path));
  document.querySelector("#app").innerHTML = view();
}

export function start() {
  window.addEventListener("popstate", () => render(location.pathname));
  document.addEventListener("click", (event) => {
    const link = event.target.closest("a[data-link]");
    if (!link) return;
    event.preventDefault();
    history.pushState({}, "", link.href);
    render(location.pathname);
  });
  render(location.pathname);
}

start();
JS
cat > projects/website/src/components/header.js <<'JS'
export const header = (title) => `<header><h1>${title}</h1></header>`;
JS
cat > projects/website/index.html <<'HTML'
<!doctype html>
<title>website</title>
<p id="hello"></p>
<script type="module" src="src/app.js"></script>
HTML

cat > projects/api/README.md <<'MD'
# api

A tiny HTTP API that stores notes.

    go run ./cmd/server

GET /notes lists them, POST /notes adds one.
MD
cat > projects/api/cmd/server/main.go <<'GO'
// Command server serves the notes API on :8080.
package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type note struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

var notes = []note{{1, "buy milk"}, {2, "call the bank"}}

func main() {
	http.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(notes)
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
GO
printf 'module example.com/api\n\ngo 1.25\n' > projects/api/go.mod
printf 'package store\n' > projects/api/internal/store/store.go
printf 'package handlers\n' > projects/api/internal/handlers/notes.go
printf '#!/bin/sh\necho backing up\n' > projects/scripts/backup.sh

printf '# Todo\n\n- [ ] write the release notes\n- [ ] update the website\n- [x] fix the api tests\n' > notes/todo.md
printf '# Ideas\n\n- a dark theme for the website\n- search in the api\n' > notes/ideas.md
printf '# Report\n\nA sample document.\n' > documents/report.md
head -c 4096 /dev/zero > photos/2025/photo-001.jpg
head -c 4096 /dev/zero > photos/2026/photo-002.jpg
head -c 4096 /dev/zero > downloads/sample.zip

# Two repos, one with unsaved changes. The demo identity is generic.
repo() {
  git -C "$1" init -q -b main
  git -C "$1" add -A
  git -C "$1" -c user.name=demo -c user.email=demo@example.com commit -qm "first commit"
}
repo projects/website
repo projects/api
git -C projects/api checkout -qb add-search
printf '\nSee CHANGELOG.md for what is new.\n' >> projects/website/README.md

# Ages, so each row's dimmed age reads like a real home.
stamp() { # stamp AGE PATH...: set the mtime AGE ago (5M, 3H, 2d, 40d)
  age=$1; shift
  case $age in *M) secs=$((${age%M} * 60));; *H) secs=$((${age%H} * 3600));; *d) secs=$((${age%d} * 86400));; esac
  t=$(($(date +%s) - secs))
  when=$(date -r "$t" +%Y%m%d%H%M.%S 2>/dev/null || date -d "@$t" +%Y%m%d%H%M.%S)
  touch -t "$when" "$@"
}
stamp 40d music photos/2025
stamp 12d photos photos/2026 documents
stamp 3d projects/scripts downloads
stamp 5H notes
stamp 2H projects/api projects/api/cmd projects/api/internal
stamp 8M projects/website projects/website/src
stamp 1H projects

# Pins and a visit log under the XDG state dir the demo shell uses.
state=$dir/state/ecd
mkdir -p "$state"
printf '%s\n' "$home/projects" "$home/notes" > "$state/pins"
now=$(date +%s)
visit() { # visit SECONDS_AGO DIR
  printf '%s\t%s\n' $((now - $1)) "$home/$2" >> "$state/visits"
}
for s in 200 900 4000 9000 30000; do visit $s projects/website; done
for s in 600 7000 20000 90000; do visit $s projects/api; done
for s in 3000 50000; do visit $s documents; done
for s in 100000 400000; do visit $s photos; done
visit 200000 downloads
visit 500000 music

# The demo shell: neutral prompt, the ecd init line, nothing else.
mkdir -p "$dir/zsh"
cat > "$dir/zsh/.zshrc" <<'ZSH'
cd ~
PROMPT='%F{blue}%~%f %F{magenta}❯%f '
unsetopt BEEP
eval "$(ecd init zsh)"
ZSH
