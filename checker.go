package main

// TODO: rewrite this code to operate just within the scope of
// individual compilation units.

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/thanm/dwarf-check/dwexaminer"
)

type readLineMode int

const (
	noReadLine     readLineMode = 0
	silentReadLine readLineMode = 1
	dumpReadLine   readLineMode = 2
)

type dumpTypesMode int

const (
	noDumpTypes  dumpTypesMode = 0
	yesDumpTypes dumpTypesMode = 1
)

type dumpBuildIdMode int

const (
	noDumpBuildId  dumpBuildIdMode = 0
	yesDumpBuildId dumpBuildIdMode = 1
)

type doAbsChecksMode int

const (
	noDoAbsChecks  doAbsChecksMode = 0
	yesDoAbsChecks doAbsChecksMode = 1
)

type dumpSizeMode int

const (
	noDumpSize     dumpSizeMode = 0
	yesDumpSize    dumpSizeMode = 1
	detailDumpSize dumpSizeMode = 2
)

type options struct {
	db dumpBuildIdMode
	dt dumpTypesMode
	rl readLineMode
	sz dumpSizeMode
	dc doAbsChecksMode
}

func readAligned4(r io.Reader, sz int32) ([]byte, error) {
	full := (sz + 3) &^ 3
	data := make([]byte, full)
	_, err := io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	data = data[:sz]
	return data, nil
}

func dumpBuildIdsInNoteSection(ef *elf.File, sect *elf.Section) error {
	const wantNameSize = 4 // GNU\0
	var wantNoteName = [...]byte{'G', 'N', 'U', 0}
	const wantNoteType = 3 // NT_GNU_BUILD_ID
	r := sect.Open()
	for {
		var namesize, descsize, noteType int32
		if err := binary.Read(r, ef.ByteOrder, &namesize); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if namesize != wantNameSize {
			continue
		}
		if err := binary.Read(r, ef.ByteOrder, &descsize); err != nil {
			return err
		}
		if err := binary.Read(r, ef.ByteOrder, &noteType); err != nil {
			return err
		}
		if noteType != wantNoteType {
			continue
		}
		noteName, err := readAligned4(r, namesize)
		if err != nil {
			continue
		}
		if wantNoteName[0] != noteName[0] ||
			wantNoteName[1] != noteName[1] ||
			wantNoteName[2] != noteName[2] ||
			wantNoteName[3] != noteName[3] {
			continue
		}
		desc, err := readAligned4(r, descsize)
		if err != nil {
			return err
		}
		descStr := hex.EncodeToString(desc)
		fmt.Fprintf(os.Stderr, "found build id '%s' in section `%s` notename `%s`\n", descStr, sect.Name, string(noteName))
	}
}

func dumpBuildId(ef *elf.File) {
	for _, sect := range ef.Sections {
		if sect.Type != elf.SHT_NOTE {
			continue
		}
		dumpBuildIdsInNoteSection(ef, sect)
	}
}

// isDebugSect returns TRUE if this section contains DWARF/debug info.
// Here we include ".eh_frame" and related, since they are basically DWARF
// under the covers.
func isDwarfSect(secName string) bool {
	return strings.HasPrefix(secName, ".debug_") ||
		strings.HasPrefix(secName, ".zdebug_") ||
		strings.HasPrefix(secName, ".eh_frame")
}

func dumpSizes(ef *elf.File, mode dumpSizeMode) {
	totDw := uint64(0)
	totExe := uint64(0)
	for _, sect := range ef.Sections {
		totExe += sect.FileSize
		if isDwarfSect(sect.Name) {
			totDw += sect.FileSize
		}
	}
	perc := func(v, tot uint64) string {
		if v == 0 || tot == 0 {
			return "0%"
		}
		fv := float64(v)
		ft := float64(tot)
		return fmt.Sprintf("%2.2f%%", (fv/ft)*100.0)
	}
	if mode == detailDumpSize {
		for _, sect := range ef.Sections {
			if isDwarfSect(sect.Name) {
				fmt.Printf("section %15s: %10d bytes, %s of DWARF, %s of exe\n",
					sect.Name, sect.FileSize,
					perc(sect.FileSize, totDw),
					perc(sect.FileSize, totExe))
			} else { // if sect.Name == ".text" {
				fmt.Printf("section %15s: %10d bytes, %s of exe\n",
					sect.Name, sect.FileSize,
					perc(sect.FileSize, totExe))
			}
		}
	}
	if mode == detailDumpSize {
		fmt.Printf("DWARF size total: %d bytes, %s of exe\n", totDw, perc(totDw, totExe))
		fmt.Printf("Exe size total: %d bytes\n", totExe)
	} else {
		fmt.Printf("DWARF size total: %d bytes\n", totDw)
	}
}

func examineFile(filename string, o options) bool {

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
				rv, err := f.DWARF()
				if o.sz != noDumpSize {
					dumpSizes(f, o.sz)
				}
				if o.db == yesDumpBuildId {
					dumpBuildId(f)
				}
				return rv, err
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
		verb(1, "loading %s for %s", try.flav, filename)
		d, derr = try.opener(filename)
		if derr != nil {
			warn("unable to open %s as %s: %v\n",
				filename, try.flav, derr)
			continue
		}
		break
	}
	if d == nil {
		return false
	}

	// Initialize state
	verb(1, "examining DWARF for %s", filename)
	rdr := d.Reader()
	var ds *dwexaminer.DwExaminer
	if o.dc != noDoAbsChecks || o.dt != noDumpTypes {
		var err error
		ds, err = dwexaminer.NewDwExaminer(rdr)
		if err != nil {
			warn("error initializing dwarf state examiner: %v", err)
			return false
		}
	}

	dcount := 0
	absocount := 0

	typeNames := make(map[string]struct{})

	isTypeTag := func(t dwarf.Tag) bool {
		switch t {
		case dwarf.TagSubrangeType, dwarf.TagSubroutineType, dwarf.TagArrayType, dwarf.TagBaseType, dwarf.TagPointerType, dwarf.TagStructType, dwarf.TagTypedef:
			return true
		}
		return false
	}

	if o.dc != noDoAbsChecks || o.dt != noDumpTypes {
		// Walk DIEs
		dieOffsets := ds.DieOffsets()
		for idx, off := range dieOffsets {
			verb(3, "examining DIE at offset 0x%x", off)
			die, err := ds.LoadEntryByOffset(off)
			dcount++
			if err != nil {
				warn("error examining DWARF: %v", err)
				return false
			}

			if o.dt != noDumpTypes {
				if isTypeTag(die.Tag) {
					if name, ok := die.Val(dwarf.AttrName).(string); ok {
						typeNames[name] = struct{}{}
					}
				}
			}

			// Does it have an abstract origin?
			ooff, originOK := die.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset)
			if !originOK {
				continue
			}
			absocount++

			// All abstract origin references should be resolvable.
			var entry *dwarf.Entry
			entry, err = ds.LoadEntryByOffset(ooff)
			if err != nil || entry == nil {
				warn("unresolved abstract origin ref from DIE %d at offset 0x%x to bad offset 0x%x\n", idx, off, ooff)
				err := ds.DumpEntry(idx, false, true, 0)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
				return false
			}
		}
		verb(1, "read %d DIEs, processed %d abstract origin refs",
			dcount, absocount)
	}

	if o.dt != noDumpTypes {
		fmt.Println("Types:")
		sl := make([]string, 0, len(typeNames))
		for k := range typeNames {
			sl = append(sl, k)
		}
		sort.Strings(sl)
		for i := range sl {
			fmt.Println(i, sl[i])
		}
	}

	if o.rl != noReadLine {
		dr := d.Reader()
		for {
			ent, err := dr.Next()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				return false
			} else if ent == nil {
				break
			}
			if ent.Tag != dwarf.TagCompileUnit {
				dr.SkipChildren()
				continue
			}

			// Decode CU's line table.
			lr, err := d.LineReader(ent)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				return false
			} else if lr == nil {
				continue
			}

			for {
				var line dwarf.LineEntry
				err := lr.Next(&line)
				if err != nil {
					if err == io.EOF {
						break
					}
					fmt.Fprintf(os.Stderr, "%v\n", err)
					return false
				}
				if o.rl == dumpReadLine {
					fmt.Printf("Address: %x File: %s Line: %d IsStmt: %v PrologueEnd: %v\n", line.Address, line.File.Name, line.Line, line.IsStmt, line.PrologueEnd)
				}
			}
		}
	}

	return true
}
