package picker

import (
	"cmp"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// indexDepth is how many levels under home the folder index reaches.
const indexDepth = 4

// skipped are folder names the index never enters: build output and
// dependency trees. Hidden folders and bundles are skipped by shape.
var skipped = []string{"Library", "node_modules", "build", "dist", "target", "out", "vendor", "__pycache__"}

// bundles are the suffixes of macOS app, photo and framework bundles.
var bundles = []string{".app", ".photoslibrary", ".framework", ".bundle"}

func skip(name string) bool {
	return strings.HasPrefix(name, ".") || slices.Contains(skipped, name) ||
		slices.ContainsFunc(bundles, func(s string) bool { return strings.HasSuffix(name, s) })
}

// cacheFile is the folder index on disk: $XDG_CACHE_HOME/ecd/folders, or
// ~/.cache/ecd/folders when it is unset. It holds one folder path per line
// and nothing else.
func cacheFile() string { return filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), "folders") }

// index is the folder index of home. It loads from the cache at once and
// refreshes from disk in the background; done closes when the refresh has
// been written back.
type index struct {
	done    chan struct{}
	folders []string // the refreshed folders, readable once done is closed
}

// loadIndex returns the cached folder index plus extra, and starts the
// refresh, whose folders also end with extra.
func loadIndex(extra []string) ([]string, *index) {
	// Paths are fixed now: the refresh may outlive whoever set HOME.
	home, _ := os.UserHomeDir()
	file := cacheFile()
	data, _ := os.ReadFile(file)
	cached := slices.DeleteFunc(strings.Split(string(data), "\n"), func(s string) bool { return s == "" })
	ix := &index{done: make(chan struct{})}
	go func() {
		defer close(ix.done)
		found := scan(home)
		saveIndex(file, found)
		ix.folders = union(found, extra)
	}()
	return union(cached, extra), ix
}

// scan lists the folders under home, indexDepth levels deep.
func scan(home string) []string {
	var folders []string
	filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == home {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if skip(d.Name()) {
			return filepath.SkipDir
		}
		folders = append(folders, path)
		if levels(path, home) >= indexDepth {
			return filepath.SkipDir
		}
		return nil
	})
	return folders
}

// saveIndex writes folders to the cache through a temp file, so a picker
// opening meanwhile never reads half an index.
func saveIndex(file string, folders []string) {
	if os.MkdirAll(filepath.Dir(file), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), "folders-*")
	if err != nil {
		return
	}
	_, err = tmp.WriteString(strings.Join(folders, "\n") + "\n")
	if tmp.Close() != nil || err != nil {
		os.Remove(tmp.Name())
		return
	}
	os.Rename(tmp.Name(), file)
}

// match is one search result: a folder, its score and the positions of the
// matched letters in its name.
type match struct {
	path  string
	score int
	hits  []int
}

const maxResults = 30

// search ranks folders against query: exact name 1000, starts-with 600,
// contains 400, tight fuzzy 100, plus the visit bonus, minus 12 per path
// level, plus 150 for a direct child of dir and 40 for a deeper one. Fuzzy
// matches show only when fewer than 5 real ones exist.
func search(query, dir string, folders []string, bonus map[string]int) []match {
	q := []rune(strings.ToLower(query))
	home, _ := os.UserHomeDir()
	var real, fuzzy []match
	for _, path := range folders {
		name := []rune(strings.Map(unicode.ToLower, filepath.Base(path)))
		m, loose := match{path: path}, false
		switch i := runeIndex(name, q); {
		case slices.Equal(name, q):
			m.score, m.hits = 1000, span(0, len(q))
		case i == 0:
			m.score, m.hits = 600, span(0, len(q))
		case i > 0:
			m.score, m.hits = 400, span(i, len(q))
		default:
			if m.hits = tight(q, name); m.hits == nil {
				continue
			}
			m.score, loose = 100, true
		}
		m.score += bonus[path] - 12*levels(path, home)
		if rel, err := filepath.Rel(dir, path); err == nil && rel != "." && filepath.IsLocal(rel) {
			if strings.ContainsRune(rel, filepath.Separator) {
				m.score += 40
			} else {
				m.score += 150
			}
		}
		if loose {
			fuzzy = append(fuzzy, m)
		} else {
			real = append(real, m)
		}
	}
	if len(real) < 5 {
		real = append(real, fuzzy...)
	}
	slices.SortFunc(real, func(a, b match) int {
		return cmp.Or(cmp.Compare(b.score, a.score), strings.Compare(a.path, b.path))
	})
	return real[:min(len(real), maxResults)]
}

// levels is how deep path sits: counted from home when under it, from /
// otherwise.
func levels(path, home string) int {
	if rel, err := filepath.Rel(home, path); err == nil && filepath.IsLocal(rel) {
		path = rel
	}
	return len(strings.FieldsFunc(path, func(r rune) bool { return r == filepath.Separator }))
}

// runeIndex is the rune position of q in name, or -1.
func runeIndex(name, q []rune) int {
	for i := 0; i+len(q) <= len(name); i++ {
		if slices.Equal(name[i:i+len(q)], q) {
			return i
		}
	}
	return -1
}

func span(from, n int) []int {
	hits := make([]int, n)
	for i := range hits {
		hits[i] = from + i
	}
	return hits
}

// tight finds q's letters in order in name within the shortest window, and
// returns their positions when that window spans at most len(q)+2 letters.
// Queries under 3 letters never match loosely.
func tight(q, name []rune) []int {
	if len(q) < 3 {
		return nil
	}
	var best []int
	for start, r := range name {
		if r != q[0] {
			continue
		}
		hits := []int{start}
		for k := start + 1; k < len(name) && len(hits) < len(q); k++ {
			if name[k] == q[len(hits)] {
				hits = append(hits, k)
			}
		}
		if len(hits) == len(q) && (best == nil || hits[len(hits)-1]-start < best[len(best)-1]-best[0]) {
			best = hits
		}
	}
	if best == nil || best[len(best)-1]-best[0]+1 > len(q)+2 {
		return nil
	}
	return best
}
