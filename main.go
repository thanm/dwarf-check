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
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

var verbflag = flag.Int("v", 0, "Verbose trace output level")
var iterflag = flag.Int("iters", 1, "Number of iterations")
var memprofileflag = flag.String("memprofile", "", "write memory profile to `file`")
var memprofilerateflag = flag.Int64("memprofilerate", 0, "set runtime.MemProfileRate to `rate`")
var cpuprofileflag = flag.String("cpuprofile", "", "write CPU profile to `file`")
var psmflag = flag.String("psm", "", "write /proc/self/maps to `file`")

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
	if *cpuprofileflag != "" {
		f, err := os.Create(*cpuprofileflag)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		verb(1, "initiating CPU profile")
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		atExit(func() {
			verb(1, "finishing CPU profile")
			pprof.StopCPUProfile()
		})
	}
	if *psmflag != "" {
		f, err := os.Create(*psmflag)
		if err != nil {
			log.Fatal("could not open -psm file: ", err)
		}
		verb(1, "collecting /proc/self/maps")
		content, err := ioutil.ReadFile("/proc/self/maps")
		if err != nil {
			log.Fatal(err)
		}
		if _, err := f.Write(content); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
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
