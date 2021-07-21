package dwexaminer_test

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thanm/dwarf-check/dwexaminer"
)

const noExtra = "-gcflags="
const moreInlExtra = "-gcflags=all=-l=4"

func buildSelf(t *testing.T, tdir string, extra string) string {
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
	tmpdir := t.TempDir()

	// build fixture
	exe := filepath.Join(tmpdir, "out.exe")
	infile := "./testdata/example.go"
	gotoolpath := filepath.Join(runtime.GOROOT(), "bin", "go")
	cmd := exec.Command(gotoolpath, "build", "-o", exe, infile)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Logf("build: %s\n", b)
		t.Fatalf("build error: %v", err)
	}

	var d *dwarf.Data
	var derr error

	tries := []struct {
		opener func(exe string) (*dwarf.Data, error)
		flav   string
	}{
		{flav: "ELF",
			opener: func(exe string) (*dwarf.Data, error) {
				f, err := elf.Open(exe)
				if err != nil {
					return nil, err
				}
				return f.DWARF()
			},
		},
		{
			flav: "Macho",
			opener: func(exe string) (*dwarf.Data, error) {
				f, err := macho.Open(exe)
				if err != nil {
					return nil, err
				}
				return f.DWARF()
			},
		},
		{
			flav: "PE",
			opener: func(exe string) (*dwarf.Data, error) {
				f, err := pe.Open(exe)
				if err != nil {
					return nil, err
				}
				return f.DWARF()
			},
		},
	}

	for _, try := range tries {
		d, derr = try.opener(exe)
		if derr != nil {
			t.Logf("unable to open %s as %s: %v\n",
				exe, try.flav, derr)
			continue
		}
		break
	}
	if d == nil {
		t.Fatalf("unable to open %s\n", exe)
	}

	// Create DWARF reader
	t.Logf("loading DWARF for %s", exe)
	if derr != nil {
		t.Fatalf("error reading DWARF: %v", derr)
	}
	rdr := d.Reader()

	// Construct an examiner.
	dwx, dwxerr := dwexaminer.NewDwExaminer(rdr)
	if dwxerr != nil {
		t.Fatalf("error reading DWARF: %v", dwxerr)
	}

	// Walk subprogram DIEs
	cname := ""
	foundABC := false
	foundruntimemain := false
	cucount := 0
	fncount := 0
	runtimecus := 0
	dieOffsets := dwx.DieOffsets()
	for idx := 0; idx < len(dieOffsets); idx++ {
		off := dieOffsets[idx]
		t.Logf("visit off %x\n", off)
		die, err := dwx.LoadEntryByOffset(off)
		if err != nil {
			t.Fatalf("LoadEntryByOffset error reading DWARF: %v", err)
		}
		if die.Tag == dwarf.TagCompileUnit {
			cucount++
			if name, ok := die.Val(dwarf.AttrName).(string); ok {
				cname = name
				if cname == "runtime" {
					runtimecus++
				}
			}
			continue
		}
		if die.Tag == dwarf.TagSubprogram {
			fncount++
			if name, ok := die.Val(dwarf.AttrName).(string); ok {
				if name == "main.ABC" {
					foundABC = true
				}
				if name == "runtime.main" {
					foundruntimemain = true
				}
			}
		}

		nidx, err := dwx.SkipChildren()
		if err != nil {
			t.Fatalf("skipChildren error reading DWARF: %v", err)
		}
		if nidx == -1 {
			// EOF
			t.Logf("EOF after cu %s, cucount=%d fncount=%d\n",
				cname, cucount, fncount)
			break
		}
		// back up by 1 to allow for increment in for loop above
		idx = nidx - 1
	}
	t.Logf("found %d compilation units, %d functions\n", cucount, fncount)

	if runtimecus == 0 {
		t.Errorf("no runtime CUs found")
	}
	if !foundABC {
		t.Errorf("subprogram main.ABC not found")
	}
	if !foundruntimemain {
		t.Errorf("subprogram runtime.main not found")
	}
}
