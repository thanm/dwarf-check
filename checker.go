package main

import (
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"os"
)

type dwstate struct {
	reader     *dwarf.Reader
	cur        *dwarf.Entry
	dieOffsets []dwarf.Offset
	kids       map[int][]int
	parent     map[int]int
}

// Given a DWARF reader, initialize dwstate
func (ds *dwstate) initialize(rdr *dwarf.Reader) error {
	ds.reader = rdr
	ds.kids = make(map[int][]int)
	ds.parent = make(map[int]int)
	var lastOffset dwarf.Offset
	var nstack []int
	for entry, err := rdr.Next(); entry != nil; entry, err = rdr.Next() {
		if err != nil {
			return err
		}
		lastOffset = entry.Offset
		idx := len(ds.dieOffsets)
		ds.dieOffsets = append(ds.dieOffsets, lastOffset)
		if entry.Tag == 0 {
			// terminator
			if len(nstack) == 0 {
				return errors.New(fmt.Sprintf("malformed dwarf at offset %v: nstack underflow", lastOffset))
			}
			nstack = nstack[:len(nstack)-1]
			continue
		}
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
		return errors.New("missing terminator")
	}
	return nil
}

func (ds *dwstate) loadEntryById(idx int) (*dwarf.Entry, error) {
	return ds.loadEntryByOffset(ds.dieOffsets[idx])
}

func (ds *dwstate) loadEntryByOffset(off dwarf.Offset) (*dwarf.Entry, error) {

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
			return nil, errors.New(fmt.Sprintf("loadEntryByOffset(%v): DWARF reader returns error %v from Next()", off, err))
		}
		ds.cur = die
		if ds.cur.Offset == off {
			return ds.cur, nil
		}
	}

	// Fall back to seek
	verb(1, "seeking to offset 0x%x", off)
	ds.reader.Seek(off)
	entry, err := ds.reader.Next()
	if err != nil {
		ds.cur = nil
		ds.reader.Seek(off)
		return nil, errors.New(fmt.Sprintf("loadEntryByOffset(%v): DWARF reader returns error %v following Seek()/Next()", off, err))
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
	entry, err := ds.loadEntryById(idx)
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
		die, err := ds.loadEntryById(k)
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
		return ret, errors.New(fmt.Sprintf("no parent entry for DIE 0x%x", off))
	}
	die, err := ds.loadEntryById(p)
	if err != nil {
		return ret, err
	}
	return die, nil
}

func examineFile(filename string) {

	verb(1, "loading ELF for %s", filename)
	f, err := elf.Open(filename)
	if err != nil {
		warn("unable to open %s: %v\n", filename, err)
		return
	}

	// Create DWARF reader
	verb(1, "loading DWARF for %s", filename)
	d, err := f.DWARF()
	if err != nil {
		warn("error reading DWARF: %v", err)
		return
	}

	// Initialize state
	verb(1, "examining DWARF for %s", filename)
	rdr := d.Reader()
	ds := dwstate{}
	ds.initialize(rdr)

	dcount := 0
	absocount := 0

	// Walk DIEs
	for idx, off := range ds.dieOffsets {
		die, err := ds.loadEntryByOffset(off)
		dcount += 1
		if err != nil {
			warn("error examining DWARF: %v", err)
			return
		}

		// Does it have an abstract origin?
		ooff, originOK := die.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset)
		if !originOK {
			continue
		}
		absocount += 1

		// All abstract origin references should be resolvable.
		_, err = ds.loadEntryByOffset(ooff)
		if err != nil {
			warn("unresolved abstract origin ref from DIE %d at offset 0x%x to bad offset 0x%x\n", idx, off, ooff)
			err := ds.dumpEntry(idx, false, true, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			return
		}
	}
	verb(1, "read %d DIEs, processed %d abstract origin refs", dcount, absocount)
}
