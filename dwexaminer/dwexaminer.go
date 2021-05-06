package dwexaminer

import (
	"debug/dwarf"
	"fmt"
	"os"
)

// TODO: rewrite this code to operate just within the scope of
// individual compilation units.

type DwExaminer struct {
	reader      *dwarf.Reader
	cur         *dwarf.Entry
	idxByOffset map[dwarf.Offset]int
	dieOffsets  []dwarf.Offset
	kids        map[int][]int
	parent      map[int]int
}

func NewDwExaminer(rdr *dwarf.Reader) (*DwExaminer, error) {
	ds := DwExaminer{}
	ds.reader = rdr
	ds.kids = make(map[int][]int)
	ds.parent = make(map[int]int)
	ds.idxByOffset = make(map[dwarf.Offset]int)
	var lastOffset dwarf.Offset
	var nstack []int
	for entry, err := rdr.Next(); entry != nil; entry, err = rdr.Next() {
		if err != nil {
			return nil, err
		}
		if entry.Tag == 0 {
			// terminator
			if len(nstack) == 0 {
				return nil, fmt.Errorf("malformed dwarf at offset %v: nstack underflow", lastOffset)
			}
			nstack = nstack[:len(nstack)-1]
			continue
		}
		if _, found := ds.idxByOffset[entry.Offset]; found {
			return nil, fmt.Errorf("DIE clash on offset 0x%x", entry.Offset)
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
		return nil, fmt.Errorf("missing terminator, lastOffset=%x", lastOffset)
	}
	return &ds, nil
}

func (ds *DwExaminer) DieOffsets() []dwarf.Offset {
	rv := make([]dwarf.Offset, len(ds.dieOffsets))
	copy(rv, ds.dieOffsets)
	return rv
}

func (ds *DwExaminer) LoadEntryByID(idx int) (*dwarf.Entry, error) {
	return ds.LoadEntryByOffset(ds.dieOffsets[idx])
}

func (ds *DwExaminer) LoadEntryByOffset(off dwarf.Offset) (*dwarf.Entry, error) {
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

func (ds *DwExaminer) SkipChildren() (int, error) {
	if ds.cur == nil {
		return -1, fmt.Errorf("misuse of SkipChildren (no current die)")
	}
	doff := ds.cur.Offset
	ds.reader.SkipChildren()
	for {
		nxt, err := ds.reader.Next()
		if err != nil {
			return -1, err
		}
		if nxt == nil {
			return -1, nil
		}
		if nidx, ok := ds.idxByOffset[nxt.Offset]; ok {
			ds.cur = nxt
			return nidx, nil
		}
		// end of CU? advance
		if nxt.Tag == 0 {
			continue
		}
		// ???
		return -1, fmt.Errorf("SkipChildren() from die at offset %x landed on unknown DIE with offset %x (%+v)\n", doff, nxt.Offset, nxt)
	}
}

func indent(ilevel int) {
	for i := 0; i < ilevel; i++ {
		fmt.Fprintf(os.Stderr, "  ")
	}
}

func (ds *DwExaminer) DumpEntry(idx int, dumpKids bool, dumpParent bool, ilevel int) error {
	entry, err := ds.LoadEntryByID(idx)
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
			ds.DumpEntry(k, true, false, ilevel+2)
		}
	}
	if dumpParent {
		p, ok := ds.parent[idx]
		if ok {
			fmt.Fprintf(os.Stderr, "\nParent:\n")
			ds.DumpEntry(p, false, false, ilevel)
		}
	}
	return nil
}

func (ds *DwExaminer) Children(idx int) ([]*dwarf.Entry, error) {
	sl := ds.kids[idx]
	ret := make([]*dwarf.Entry, len(sl))
	for i, k := range sl {
		die, err := ds.LoadEntryByID(k)
		if err != nil {
			return ret, err
		}
		ret[i] = die
	}
	return ret, nil
}

// Returns parent DIE for DIE 'idx', or nil if the DIE is top level
func (ds *DwExaminer) Parent(idx int) (*dwarf.Entry, error) {
	var ret *dwarf.Entry
	off := ds.dieOffsets[idx]
	p, found := ds.parent[idx]
	if !found {
		return ret, fmt.Errorf("no parent entry for DIE 0x%x", off)
	}
	die, err := ds.LoadEntryByID(p)
	if err != nil {
		return ret, err
	}
	return die, nil
}
