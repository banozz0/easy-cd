package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// open starts ecd in dir with stdout captured and returns how long it took
// to draw its first frame, the first folder of dir on screen. It then waits
// for until, when set, and quits.
func open(t *testing.T, dir string, env []string, until func() bool) time.Duration {
	t.Helper()
	cmd := exec.Command(ecd(t))
	cmd.Dir, cmd.Env = dir, env
	// The shell function captures stdout, as here: on a terminal Bubble Tea's
	// init would first ask it for its background colour.
	cmd.Stdout = io.Discard
	start := time.Now()
	_, quit := onTerminal(t, cmd, "d00")
	took := time.Since(start)
	for deadline := time.Now().Add(5 * time.Second); until != nil && !until(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("open never finished priming")
		}
	}
	quit()
	return took
}

// onTerminal starts cmd on an 80x24 terminal and waits until it has drawn
// marker. It returns the bytes the terminal got by then, and quit, which
// presses Ctrl+C and waits for cmd to exit.
func onTerminal(t *testing.T, cmd *exec.Cmd, marker string) (screen []byte, quit func()) {
	t.Helper()
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	drawn := make(chan []byte, 1)
	go func() {
		var seen []byte
		buf := make([]byte, 4096)
		for !bytes.Contains(seen, []byte(marker)) {
			n, err := tty.Read(buf)
			if err != nil {
				return
			}
			seen = append(seen, buf[:n]...)
		}
		drawn <- seen
		io.Copy(io.Discard, tty) // keep reading so ecd never blocks drawing
	}()
	quit = func() {
		tty.Write([]byte{3}) // Ctrl+C
		cmd.Wait()
		tty.Close()
	}
	select {
	case screen = <-drawn:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		tty.Close()
		t.Fatalf("%q not drawn in 5s", marker)
	}
	return screen, quit
}

// TestPickerDrawsInColourThroughTheShellFunction runs ecd the way a user
// does, through the zsh function, whose command substitution captures
// stdout, on a terminal that says it shows true colour. The picker must
// draw in colour on that terminal: the breadcrumb in the theme's blue.
func TestPickerDrawsInColourThroughTheShellFunction(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("zsh", "-f", "-c", fmt.Sprintf(`eval "$('%s' init zsh)"; ecd`, ecd(t)))
	cmd.Dir = home
	cmd.Env = []string{"HOME=" + home, "PATH=" + filepath.Dir(ecd(t)) + ":/usr/bin:/bin",
		"TERM=xterm-256color", "COLORTERM=truecolor",
		"XDG_STATE_HOME=" + filepath.Join(root, "state"), "XDG_CACHE_HOME=" + filepath.Join(root, "cache")}

	screen, quit := onTerminal(t, cmd, "quit") // the last key in the help
	quit()

	if blue := "\x1b[1;38;2;137;179;250m"; !bytes.Contains(screen, []byte(blue)) { // #89b4fa, bold
		t.Fatalf("breadcrumb not drawn in the theme's blue; the terminal got %q", strings.ToValidUTF8(string(screen), "?"))
	}
}
