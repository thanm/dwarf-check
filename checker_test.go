package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBasic(t *testing.T) {
	*verbflag = 1

	// Avoid using /proc/self/exe if this is a temporary executable,
	// since "go test" passes -w to the linker.
	exe := "/proc/self/exe"
	linked, err := filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s) failed: %v", exe, err)
	}
	if strings.HasPrefix(linked, "/tmp") && strings.HasSuffix(linked, "dwarf-check.test") {
		// Create tempdir
		dir, err := ioutil.TempDir("", "BasicDwarfCheckTest")
		if err != nil {
			t.Fatalf("could not create directory: %v", err)
		}
		defer os.RemoveAll(dir)

		// Do a build of . into <tmpdir>/out.exe
		exe = filepath.Join(dir, "out.exe")
		gotoolpath := filepath.Join(runtime.GOROOT(), "bin", "go")
		cmd := exec.Command(gotoolpath, "build", "-o", exe, ".")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Logf("build: %s\n", b)
			t.Fatalf("build error: %v", err)
		}
	}

	fmt.Fprintf(os.Stderr, "exe is %s\n", exe)

	// Now examine the result.
	res := examineFile(exe)
	if !res {
		t.Errorf("examineFile returned false")
	}
}
