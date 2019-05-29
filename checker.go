package main

// TODO: rewrite this code to operate just within the scope of
// individual compilation units.

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"fmt"
	"io"
	"os"
)

type dwstate struct {
	reader      *dwarf.Reader
	cur         *dwarf.Entry
	idxByOffset map[dwarf.Offset]int
	dieOffsets  []dwarf.Offset
	kids        map[int][]int
	parent      map[int]int
}

// Given a DWARF reader, initialize dwstate
func (ds *dwstate) initialize(rdr *dwarf.Reader) error {
	ds.reader = rdr
	ds.kids = make(map[int][]int)
	ds.parent = make(map[int]int)
	ds.idxByOffset = make(map[dwarf.Offset]int)
	var lastOffset dwarf.Offset
	var nstack []int
	for entry, err := rdr.Next(); entry != nil; entry, err = rdr.Next() {
		verb(3, "dwstate.initialize: entry = %v", entry)
		if err != nil {
			return err
		}
		if entry.Tag == 0 {
			// terminator
			if len(nstack) == 0 {
				return fmt.Errorf("malformed dwarf at offset %v: nstack underflow", lastOffset)
			}
			nstack = nstack[:len(nstack)-1]
			continue
		}
		if _, found := ds.idxByOffset[entry.Offset]; found {
			return fmt.Errorf("DIE clash on offset 0x%x", entry.Offset)
		}
		idx := len(ds.dieOffsets)
		ds.idxByOffset[entry.Offset] = idx
		lastOffset = entry.Offset
		ds.dieOffsets = append(ds.dieOffsets, lastOffset)
		if len(nstack) > 0 {
			parent := nstack[len(nstack)-1]
			ds.kids[parent] = append(ds.kids[parent], idx)
			ds.parent[idx] = parent
		}
		if entry.Children {
			nstack = append(nstack, idx)
		}
	}
	if len(nstack) > 0 {
		return fmt.Errorf("missing terminator")
	}
	return nil
}

func (ds *dwstate) loadEntryByID(idx int) (*dwarf.Entry, error) {
	return ds.loadEntryByOffset(ds.dieOffsets[idx])
}

func (ds *dwstate) loadEntryByOffset(off dwarf.Offset) (*dwarf.Entry, error) {

	// Check to make sure this is a valid offset
	if _, found := ds.idxByOffset[off]; !found {
		return nil, fmt.Errorf("invalid offset 0x%x passed to loadEntryByOffset", off)
	}

	// Current DIE happens to be the one for which we're looking?
	if ds.cur != nil {
		if ds.cur.Offset == off {
			return ds.cur, nil
		}

		// Read next die -- maybe that is the one.
		die, err := ds.reader.Next()
		if err != nil {
			ds.reader.Seek(0)
			ds.cur = nil
			return nil, fmt.Errorf("loadEntryByOffset(%v): DWARF reader returns error %v from Next()", off, err)
		}
		ds.cur = die
		if ds.cur.Offset == off {
			return ds.cur, nil
		}
	}

	// Fall back to seek
	verb(2, "seeking to offset 0x%x", off)
	ds.reader.Seek(off)
	entry, err := ds.reader.Next()
	if err != nil {
		ds.cur = nil
		ds.reader.Seek(off)
		return nil, fmt.Errorf("loadEntryByOffset(%v): DWARF reader returns error %v following Seek()/Next()", off, err)
	}
	ds.cur = entry
	return ds.cur, nil
}

func indent(ilevel int) {
	for i := 0; i < ilevel; i++ {
		fmt.Fprintf(os.Stderr, "  ")
	}
}

func (ds *dwstate) dumpEntry(idx int, dumpKids bool, dumpParent bool, ilevel int) error {
	entry, err := ds.loadEntryByID(idx)
	if err != nil {
		return err
	}
	indent(ilevel)
	fmt.Fprintf(os.Stderr, "0x%x: %v\n", entry.Offset, entry.Tag)
	for _, f := range entry.Field {
		indent(ilevel)
		fmt.Fprintf(os.Stderr, "at=%v val=0x%x\n", f.Attr, f.Val)
	}
	if dumpKids {
		ksl := ds.kids[idx]
		for _, k := range ksl {
			ds.dumpEntry(k, true, false, ilevel+2)
		}
	}
	if dumpParent {
		p, ok := ds.parent[idx]
		if ok {
			fmt.Fprintf(os.Stderr, "\nParent:\n")
			ds.dumpEntry(p, false, false, ilevel)
		}
	}
	return nil
}

func (ds *dwstate) Children(idx int) ([]*dwarf.Entry, error) {
	sl := ds.kids[idx]
	ret := make([]*dwarf.Entry, len(sl))
	for i, k := range sl {
		die, err := ds.loadEntryByID(k)
		if err != nil {
			return ret, err
		}
		ret[i] = die
	}
	return ret, nil
}

// Returns parent DIE for DIE 'idx', or nil if the DIE is top level
func (ds *dwstate) Parent(idx int) (*dwarf.Entry, error) {
	var ret *dwarf.Entry
	off := ds.dieOffsets[idx]
	p, found := ds.parent[idx]
	if !found {
		return ret, fmt.Errorf("no parent entry for DIE 0x%x", off)
	}
	die, err := ds.loadEntryByID(p)
	if err != nil {
		return ret, err
	}
	return die, nil
}

func examineFile(filename string, readline bool) bool {

	var d *dwarf.Data
	var derr error

	verb(1, "loading ELF for %s", filename)
	f, eerr := elf.Open(filename)
	if eerr != nil {
		// Try macho
		f, merr := macho.Open(filename)
		if merr != nil {
			warn("unable to open as ELF %s: %v\n", filename, eerr)
			warn("unable to open as Mach-O %s: %v\n", filename, merr)
			return false
		}
		d, derr = f.DWARF()
	} else {
		d, derr = f.DWARF()
	}

	// Create DWARF reader
	verb(1, "loading DWARF for %s", filename)
	if derr != nil {
		warn("error reading DWARF: %v", derr)
		return false
	}

	// Initialize state
	verb(1, "examining DWARF for %s", filename)
	rdr := d.Reader()
	ds := dwstate{}
	err := ds.initialize(rdr)
	if err != nil {
		warn("error initializing dwarf state examiner: %v", err)
		return false
	}

	dcount := 0
	absocount := 0

	// Walk DIEs
	for idx, off := range ds.dieOffsets {
		verb(3, "examining DIE at offset 0x%x", off)
		die, err := ds.loadEntryByOffset(off)
		dcount++
		if err != nil {
			warn("error examining DWARF: %v", err)
			return false
		}

		// Does it have an abstract origin?
		ooff, originOK := die.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset)
		if !originOK {
			continue
		}
		absocount++

		// All abstract origin references should be resolvable.
		var entry *dwarf.Entry
		entry, err = ds.loadEntryByOffset(ooff)
		if err != nil || entry == nil {
			warn("unresolved abstract origin ref from DIE %d at offset 0x%x to bad offset 0x%x\n", idx, off, ooff)
			err := ds.dumpEntry(idx, false, true, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			return false
		}
	}
	verb(1, "read %d DIEs, processed %d abstract origin refs",
		dcount, absocount)

	if readline {
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
				fmt.Printf("Address: %x File: %s Line: %d IsStmt: %v PrologueEnd: %v\n", line.Address, line.File.Name, line.Line, line.IsStmt, line.PrologueEnd)
			}
		}
	}

	return true
}
