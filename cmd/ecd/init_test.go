package main_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// binDir holds the ecd binary the tests build.
var binDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ecd-bin")
	if err != nil {
		panic(err)
	}
	binDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// build builds ecd on first use. go test caches a pass against the files the
// test process reads, and go build reads the sources in another process, so
// they are read here too, inside a test where go test records it: editing
// any of them makes the next go test run these tests again.
var build = sync.OnceValues(func() (string, error) {
	readBuildInputs("../..")
	bin := filepath.Join(binDir, "ecd")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %v\n%s", err, out)
	}
	return bin, nil
})

// ecd is the path of the built binary.
func ecd(t *testing.T) string {
	t.Helper()
	bin, err := build()
	if err != nil {
		t.Fatal(err)
	}
	return bin
}

// readBuildInputs reads the module's non-test Go files, go.mod and go.sum
// under root, skipping hidden folders such as .git.
func readBuildInputs(root string) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch name := d.Name(); {
		case err != nil:
			return nil
		case d.IsDir() && path != root && strings.HasPrefix(name, "."):
			return filepath.SkipDir
		case name == "go.mod" || name == "go.sum" || strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go"):
			os.ReadFile(path)
		}
		return nil
	})
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
		script = fmt.Sprintf(sh.source, ecd(t)) + "; " + script
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

// TestCdDashReturnsToTheFolderBeforeThePick covers fish above all, whose
// ecd calls its cd function: cd - must still know the folder before ecd.
func TestCdDashReturnsToTheFolderBeforeThePick(t *testing.T) {
	eachShell(t, func(t *testing.T, sh shell) {
		if got := sh.run(t, t.TempDir(), 0, "ecd; cd - >/dev/null; basename $PWD"); got != "start" {
			t.Fatalf("pwd after cd - ends in %q, want start", got)
		}
	})
}

// TestInitRejectsAnUnknownShell also checks the usage names both bash
// files: macOS bash login shells read ~/.bash_profile, others ~/.bashrc.
func TestInitRejectsAnUnknownShell(t *testing.T) {
	for _, args := range [][]string{{"init"}, {"init", "tcsh"}} {
		out, err := exec.Command(ecd(t), args...).CombinedOutput()
		if err == nil {
			t.Fatalf("ecd %s exited 0:\n%s", strings.Join(args, " "), out)
		}
		for _, want := range []string{"usage:", "~/.bashrc", "~/.bash_profile"} {
			if !strings.Contains(string(out), want) {
				t.Fatalf("ecd %s usage lacks %q:\n%s", strings.Join(args, " "), want, out)
			}
		}
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
