//
// This program reads in DWARF info for a given load module (shared
// library or executable) and inspects it for problems/insanities,
// primarily abstract origin references that are incorrect.
// Can be run on object files as well, in theory.
//
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

var verbflag = flag.Int("v", 0, "Verbose trace output level")
var iterflag = flag.Int("iters", 1, "Number of iterations")
var memprofileflag = flag.String("memprofile", "", "write memory profile to `file`")
var memprofilerateflag = flag.Int64("memprofilerate", 0, "set runtime.MemProfileRate to `rate`")

var st int
var atExitFuncs []func()

func atExit(f func()) {
	atExitFuncs = append(atExitFuncs, f)
}

func Exit(code int) {
	for i := len(atExitFuncs) - 1; i >= 0; i-- {
		f := atExitFuncs[i]
		atExitFuncs = atExitFuncs[:i]
		f()
	}
	os.Exit(code)
}

func verb(vlevel int, s string, a ...interface{}) {
	if *verbflag >= vlevel {
		fmt.Printf(s, a...)
		fmt.Printf("\n")
	}
}

func warn(s string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, s, a...)
	fmt.Fprintf(os.Stderr, "\n")
	st = 1
}

func usage(msg string) {
	if len(msg) > 0 {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "usage: dwarf-check [flags] <ELF files>\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func setupMemProfile() {
	if *memprofilerateflag != 0 {
		runtime.MemProfileRate = int(*memprofilerateflag)
	}
	f, err := os.Create(*memprofileflag)
	if err != nil {
		log.Fatalf("%v", err)
	}
	atExit(func() {
		// Profile all outstanding allocations.
		runtime.GC()
		const writeLegacyFormat = 1
		if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
			log.Fatalf("%v", err)
		}
	})
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("dwarf-check: ")
	flag.Parse()
	if *memprofileflag != "" {
		setupMemProfile()
	}
	verb(1, "in main")
	if flag.NArg() == 0 {
		usage("please supply one or more ELF files as command line arguments")
	}
	for _, arg := range flag.Args() {
		for i := 0; i < *iterflag; i++ {
			examineFile(arg)
		}
	}
	verb(1, "leaving main")
	Exit(st)
}
