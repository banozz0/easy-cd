package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var ecdBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ecd-bin")
	if err != nil {
		panic(err)
	}
	ecdBin = filepath.Join(dir, "ecd")
	if out, err := exec.Command("go", "build", "-o", ecdBin, ".").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// zsh evals the real `ecd init zsh` output in a clean zsh, with a stub
// picker named ecd first on PATH that prints pick and exits with exit, runs
// script in a start folder and returns its trimmed stdout.
func zsh(t *testing.T, pick string, exit int, script string) string {
	t.Helper()
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed")
	}
	tmp := t.TempDir()
	stubDir := filepath.Join(tmp, "stub")
	start := filepath.Join(tmp, "start")
	for _, d := range []string{stubDir, start} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub := "#!/bin/sh\nprintf '%s' \"$ECD_STUB_PICK\"\nexit \"$ECD_STUB_EXIT\"\n"
	if err := os.WriteFile(filepath.Join(stubDir, "ecd"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("zsh", "-f", "-c", `eval "$('`+ecdBin+`' init zsh)"; cd `+start+"; "+script)
	cmd.Env = append(os.Environ(), "PATH="+stubDir+":"+os.Getenv("PATH"), "ECD_STUB_PICK="+pick, "ECD_STUB_EXIT="+strconv.Itoa(exit))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestEcdCdsToThePickedFolder(t *testing.T) {
	target := t.TempDir()
	want := mustReal(t, target)

	if got := zsh(t, target, 0, "ecd; pwd -P"); got != want {
		t.Fatalf("pwd %q, want %q", got, want)
	}
}

func TestEcdLandsInTheFolderOfAnOpenedFile(t *testing.T) {
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "plan.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want := mustReal(t, folder)

	// Enter on plan.txt prints the file's folder, as the stub does here.
	if got := zsh(t, folder, 0, "ecd; pwd -P"); got != want {
		t.Fatalf("pwd %q, want %q", got, want)
	}
}

func TestEcdStaysPutWhenNothingIsPicked(t *testing.T) {
	if got := zsh(t, "", 0, "ecd; basename $PWD"); got != "start" {
		t.Fatalf("pwd ends in %q, want start", got)
	}
}

func TestEcdStaysPutWhenThePickerFails(t *testing.T) {
	if got := zsh(t, t.TempDir(), 1, "ecd; basename $PWD"); got != "start" {
		t.Fatalf("pwd ends in %q, want start", got)
	}
}

func TestCdIsStillTheBuiltin(t *testing.T) {
	if got := zsh(t, "", 0, "type cd"); got != "cd is a shell builtin" {
		t.Fatalf("type cd = %q", got)
	}
}

func mustReal(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
