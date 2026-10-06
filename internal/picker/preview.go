package picker

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

const (
	// previewBytes is how much of a highlighted file the preview reads.
	previewBytes = 64 << 10
	// maxFile is the size past which a file shows a notice, not content.
	maxFile = 10 << 20
)

// preview is what the preview pane shows for one file: its first lines,
// or a one-line notice when it cannot be shown.
type preview struct {
	path   string
	lines  []string
	notice string
}

// peek reads the start of the file at path for the preview. It never opens
// anything but a regular file under maxFile, so a pipe or device can't hang
// the picker.
func peek(path string) preview {
	fi, err := os.Stat(path)
	switch {
	case err != nil:
		return preview{path: path, notice: cantRead(err)}
	case !fi.Mode().IsRegular():
		return preview{path: path, notice: "not a regular file"}
	case fi.Size() > maxFile:
		return preview{path: path, notice: fmt.Sprintf("too large to preview: %d MB", fi.Size()>>20)}
	}
	f, err := os.Open(path)
	if err != nil {
		return preview{path: path, notice: cantRead(err)}
	}
	defer f.Close()
	data, _ := io.ReadAll(io.LimitReader(f, previewBytes))
	if bytes.IndexByte(data, 0) >= 0 {
		return preview{path: path, notice: "binary file"}
	}
	p := preview{path: path}
	for _, line := range strings.Split(string(data), "\n") {
		p.lines = append(p.lines, printable(line))
	}
	return p
}

// cantRead is the notice for err, without the path the pane already names.
func cantRead(err error) string {
	return "can't read: " + cmp.Or(errors.Unwrap(err), err).Error()
}

// printable is line safe to put on a terminal: tabs as spaces, invalid
// UTF-8 replaced, and control codes dropped so a file can't drive the
// terminal.
func printable(line string) string {
	line = strings.ReplaceAll(strings.ToValidUTF8(line, "�"), "\t", "    ")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, line)
}

// cells is the preview's first rows lines, a notice dimmed.
func (p preview) cells(rows int) []string {
	if p.notice != "" {
		return []string{dim.Render(p.notice)}
	}
	return p.lines[:min(rows, len(p.lines))]
}
