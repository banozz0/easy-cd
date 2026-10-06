package main_test

import (
	"fmt"
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

// shell is one shell ecd init supports, run clean with no user config.
type shell struct {
	name   string
	cmd    []string // runs the script given as the last argument
	source string   // loads the `ecd init` output; %s is the ecd binary
	cdType string   // how `type cd` starts in a clean shell
}

// shells lists them all; bash is /bin/bash, the 3.2 macOS ships, so the
// oldest bash stays covered. fish ships cd as a function over its builtin.
var shells = []shell{
	{"zsh", []string{"zsh", "-f", "-c"}, `eval "$('%s' init zsh)"`, "cd is a shell builtin"},
	{"bash", []string{"/bin/bash", "--noprofile", "--norc", "-c"}, `eval "$('%s' init bash)"`, "cd is a shell builtin"},
	{"fish", []string{"fish", "--no-config", "-c"}, `'%s' init fish | source`, "cd is a function with definition"},
}

// eachShell runs test once per shell, as a parallel subtest named after it.
func eachShell(t *testing.T, test func(t *testing.T, sh shell)) {
	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			t.Parallel()
			test(t, sh)
		})
	}
}

// run runs script in sh, after its source line when set, with a stub picker
// named ecd first on PATH that prints pick and exits with exit, in a start
// folder, and returns its trimmed stdout.
func (sh shell) run(t *testing.T, pick string, exit int, script string) string {
	t.Helper()
	if _, err := exec.LookPath(sh.cmd[0]); err != nil {
		t.Skipf("%s not installed: the %s shell bridge is untested", sh.cmd[0], sh.name)
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

	script = "cd " + start + "; " + script
	if sh.source != "" {
		script = fmt.Sprintf(sh.source, ecdBin) + "; " + script
	}
	cmd := exec.Command(sh.cmd[0], append(sh.cmd[1:], script)...)
	cmd.Env = append(os.Environ(), "PATH="+stubDir+":"+os.Getenv("PATH"), "ECD_STUB_PICK="+pick, "ECD_STUB_EXIT="+strconv.Itoa(exit))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", sh.name, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestEcdCdsToThePrintedFolder covers a picked folder, which is also what
// Enter on a file prints (its folder), and one with a space in its name.
func TestEcdCdsToThePrintedFolder(t *testing.T) {
	for name, folder := range map[string]string{"picked": "", "with spaces": "two words"} {
		t.Run(name, func(t *testing.T) {
			eachShell(t, func(t *testing.T, sh shell) {
				target := filepath.Join(t.TempDir(), folder)
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatal(err)
				}
				want := mustReal(t, target)

				if got := sh.run(t, target, 0, "ecd; pwd -P"); got != want {
					t.Fatalf("pwd %q, want %q", got, want)
				}
			})
		})
	}
}

func TestEcdStaysPut(t *testing.T) {
	for name, exit := range map[string]int{"nothing picked": 0, "picker fails": 1} {
		t.Run(name, func(t *testing.T) {
			eachShell(t, func(t *testing.T, sh shell) {
				pick := ""
				if exit != 0 {
					pick = t.TempDir() // a failed picker's output is ignored
				}
				if got := sh.run(t, pick, exit, "ecd; basename $PWD"); got != "start" {
					t.Fatalf("pwd ends in %q, want start", got)
				}
			})
		})
	}
}

func TestCdIsUntouched(t *testing.T) {
	eachShell(t, func(t *testing.T, sh shell) {
		bare := sh
		bare.source = ""
		before := bare.run(t, "", 0, "type cd")
		if after := sh.run(t, "", 0, "type cd"); after != before {
			t.Fatalf("type cd after init:\n%s\nbefore:\n%s", after, before)
		}
		if !strings.HasPrefix(before, sh.cdType) {
			t.Fatalf("type cd = %q, want it to start %q", before, sh.cdType)
		}
	})
}

func TestInitRejectsAnUnknownShell(t *testing.T) {
	out, err := exec.Command(ecdBin, "init", "tcsh").CombinedOutput()
	if err == nil {
		t.Fatalf("ecd init tcsh exited 0:\n%s", out)
	}
	if !strings.Contains(string(out), "usage:") {
		t.Fatalf("ecd init tcsh printed no usage:\n%s", out)
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
