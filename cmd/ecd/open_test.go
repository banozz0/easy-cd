package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
)

// openBudget is how long ecd may take from process start to its first frame
// on a warm cache.
const openBudget = 100 * time.Millisecond

// TestWarmOpenDrawsTheFirstFrameUnderBudget runs the real binary on a
// terminal, against a temp home of 5050 folders whose cache a first open has
// written, and times five opens from process start to the first frame.
func TestWarmOpenDrawsTheFirstFrameUnderBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("times six real opens")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	for i := range 50 {
		d := filepath.Join(home, fmt.Sprintf("d%02d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := range 100 {
			if err := os.Mkdir(filepath.Join(d, fmt.Sprintf("e%03d", j)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	env := append(os.Environ(), "HOME="+home, "TERM=xterm-256color",
		"XDG_STATE_HOME="+filepath.Join(root, "state"), "XDG_CACHE_HOME="+filepath.Join(root, "cache"))
	cache := filepath.Join(root, "cache", "ecd", "folders")

	open(t, home, env, func() bool { _, err := os.Stat(cache); return err == nil })
	var worst time.Duration
	for range 5 {
		worst = max(worst, open(t, home, env, nil))
	}
	t.Logf("worst warm open of 5: %v", worst)
	if worst > openBudget {
		t.Fatalf("worst warm open %v, budget %v", worst, openBudget)
	}
}

// open starts ecd in dir on an 80x24 terminal with stdout captured, and
// returns how long it took to draw its first frame, the first folder of dir
// on screen. It then waits for until, when set, and quits with Ctrl+C.
func open(t *testing.T, dir string, env []string, until func() bool) time.Duration {
	t.Helper()
	cmd := exec.Command(ecd(t))
	cmd.Dir, cmd.Env = dir, env
	// The shell function captures stdout, as here: on a terminal Bubble Tea's
	// init would first ask it for its background colour.
	cmd.Stdout = io.Discard
	start := time.Now()
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	defer tty.Close()

	drawn := make(chan time.Duration, 1)
	go func() {
		var screen []byte
		buf := make([]byte, 4096)
		for !bytes.Contains(screen, []byte("d00")) {
			n, err := tty.Read(buf)
			if err != nil {
				return
			}
			screen = append(screen, buf[:n]...)
		}
		drawn <- time.Since(start)
		io.Copy(io.Discard, tty) // keep reading so ecd never blocks drawing
	}()
	var took time.Duration
	select {
	case took = <-drawn:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("no first frame in 5s")
	}
	for deadline := time.Now().Add(5 * time.Second); until != nil && !until(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("open never finished priming")
		}
	}
	tty.Write([]byte{3}) // Ctrl+C
	cmd.Wait()
	return took
}
