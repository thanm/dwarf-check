package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
)

const noExtra = "-gcflags="
const moreInlExtra = "-gcflags=all=-l=4"

func buildSelf(t *testing.T, tdir string, extra string) string {
	// Do a build of . into <tmpdir>/out.exe
	exe := filepath.Join(tdir, "out.exe")
	gotoolpath := filepath.Join(runtime.GOROOT(), "bin", "go")
	cmd := exec.Command(gotoolpath, "build", extra, "-o", exe, ".")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Logf("build: %s\n", b)
		t.Fatalf("build error: %v", err)
	}
	return exe
}

func TestBasic(t *testing.T) {
	*verbflag = 1

	bip, ok := debug.ReadBuildInfo()
	if !ok {
		println("no build info")
	} else {
		println("main package path", bip.Path)
		fmt.Printf("main mod: %+v\n", bip.Main)
		for k, dep := range bip.Deps {
			fmt.Printf("  dep %d: %+v\n", k, dep)
		}
	}

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
