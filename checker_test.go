package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const noExtra = "-gcflags="
const moreInlExtra = "-gcflags=all=-l=4"

func buildSelf(t *testing.T, tdir string, extra string) string {
	exe := "/proc/self/exe"
	linked, err := filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s) failed: %v", exe, err)
	}
	if strings.HasPrefix(linked, "/tmp") && strings.HasSuffix(linked, "dwarf-check.test") {

		// Do a build of . into <tmpdir>/out.exe
		exe = filepath.Join(tdir, "out.exe")
		gotoolpath := filepath.Join(runtime.GOROOT(), "bin", "go")
		cmd := exec.Command(gotoolpath, "build", extra, "-o", exe, ".")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Logf("build: %s\n", b)
			t.Fatalf("build error: %v", err)
		}
	}
	return exe
}

func TestBasic(t *testing.T) {
	*verbflag = 1

	// Create tempdir
	dir, err := ioutil.TempDir("", "BasicDwarfTest")
	if err != nil {
		t.Fatalf("could not create directory: %v", err)
	}
	defer os.RemoveAll(dir)

	// Grab executable to work on.
	exe := buildSelf(t, dir, noExtra)

	// Now examine the result.
	basicOpt := options{rl: silentReadLine}
	res := examineFile(exe, basicOpt)
	if !res {
		t.Errorf("examineFile returned false")
	}
}
